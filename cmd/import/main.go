// cmd/import imports the legacy kids_subjects MySQL database into this
// app's Postgres schema -- see the M10 planning memory
// (coglab_import_milestone_decisions) for the full set of decisions
// behind its design.
//
// Usage:
//
//	LEGACY_MYSQL_DSN=... DATABASE_URL=... go run ./cmd/import [-dry-run=true] [-report=path]
//
// Defaults to -dry-run=true: every table is imported into a real
// transaction (so Postgres's own constraints -- FKs, enums, uniqueness
// -- do the actual validation, not a hand-rolled approximation of them)
// and then rolled back, never committed, with a report of what would
// have happened. Pass -dry-run=false to commit for real; the importer
// refuses to do so if the report contains any errors, so a partially-
// bad table can't silently import short.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	dryRun := flag.Bool("dry-run", true, "validate and report without committing (default true; pass -dry-run=false to import for real)")
	reportPath := flag.String("report", "", "write the report to this file instead of stdout")
	flag.Parse()

	legacyDSN := os.Getenv("LEGACY_MYSQL_DSN")
	if legacyDSN == "" {
		return fmt.Errorf("LEGACY_MYSQL_DSN environment variable is required")
	}
	targetURL := os.Getenv("DATABASE_URL")
	if targetURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable is required")
	}

	ctx := context.Background()

	legacyDB, err := sql.Open("mysql", legacyDSN)
	if err != nil {
		return fmt.Errorf("open legacy database: %w", err)
	}
	defer legacyDB.Close()
	if err := legacyDB.PingContext(ctx); err != nil {
		return fmt.Errorf("connect to legacy database: %w", err)
	}

	pool, err := pgxpool.New(ctx, targetURL)
	if err != nil {
		return fmt.Errorf("create target connection pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to target database: %w", err)
	}

	report := NewReport()
	importedAt := time.Now()

	if err := runImport(ctx, legacyDB, pool, report, importedAt, *dryRun); err != nil {
		return fmt.Errorf("import: %w", err)
	}

	out := io.Writer(os.Stdout)
	if *reportPath != "" {
		f, err := os.Create(*reportPath)
		if err != nil {
			return fmt.Errorf("create report file: %w", err)
		}
		defer f.Close()
		out = f
	}
	report.Write(out)

	if *dryRun {
		fmt.Println("\ndry run: nothing was committed. Re-run with -dry-run=false once this report is clean.")
		return nil
	}
	if report.HasErrors() {
		return fmt.Errorf("import had errors -- see the report; nothing after the first failing table's transaction was committed")
	}
	fmt.Println("\nimport complete.")
	return nil
}

// runImport walks every table in FK-dependency order, all inside one
// shared transaction for the whole run (not one per table): a later
// table (say, children) needs to see an earlier table's rows (families)
// to satisfy its own foreign keys, which a separate per-table
// transaction wouldn't provide once a dry run starts rolling tables
// back -- every later insert would fail on a foreign key that (from its
// own transaction's point of view) was never actually committed. One
// transaction for the whole run, decided commit-vs-rollback exactly
// once at the end, sidesteps that entirely and is why a dry run still
// executes real SQL (see the package doc comment) rather than a
// hand-rolled approximation of Postgres's own constraints: every insert
// really happens, in order, with real foreign keys enforced, right up
// until the final rollback undoes all of it at once.
//
// Each step's own per-row errors go into report and don't stop later
// steps -- but a step-level error (a query against the legacy database
// failing, a sequence bump failing, ...) is a real infrastructure
// problem, not a bad row, and does abort the whole run.
func runImport(ctx context.Context, legacyDB *sql.DB, pool *pgxpool.Pool, report *Report, importedAt time.Time, dryRun bool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := runImportSteps(ctx, legacyDB, tx, report, importedAt); err != nil {
		tx.Rollback(ctx)
		return err
	}
	if dryRun || report.HasErrors() {
		return tx.Rollback(ctx)
	}
	return tx.Commit(ctx)
}

// validateKnownEnums checks every legacy enum column this importer maps
// against its *live* definition before touching a single row -- this
// database's enums have repeatedly drifted from the create-database
// reference script over ~20 years of direct schema changes (see the
// mapping functions' own comments for the specific, real examples this
// caught), so trusting a hardcoded snapshot risks discovering a gap
// only after tens of thousands of rows have already failed with the
// same error. Aggregates every column's failures into one report
// rather than stopping at the first, so a single re-run can fix
// everything found instead of one column at a time.
func validateKnownEnums(ctx context.Context, legacyDB *sql.DB) error {
	checks := []struct {
		table, column string
		mapped        func(string) bool
	}{
		{"children", "gender", func(s string) bool { return s == "Male" || s == "Female" || s == "Unknown" }},
		{"children", "premie", func(s string) bool { return s == "Premie" || s == "Full Term" || s == "Unknown" }},
		{"children", "birth_comp", func(s string) bool { return s == "Yes" || s == "No" || s == "Unknown" }},
		{"children", "twin", func(s string) bool { return s == "Yes" || s == "No" || s == "Unknown" }},
		{"children", "ethnicity", func(s string) bool {
			return s == "Unknown" || s == "Hispanic or Latino" || s == "Not Hispanic or Latino"
		}},
		{"children", "race", func(s string) bool {
			_, ok := legacyRaceToCategory[s]
			return ok || s == "Unknown or Not Reported"
		}},
		{"children", "other_race", func(s string) bool {
			_, ok := legacyRaceToCategory[s]
			return ok || s == "Unknown or Not Reported" || s == "No other race"
		}},
		{"children", "response", func(s string) bool { _, ok := legacyResponseToText[s]; return ok }},
		{"parents", "education_1", func(s string) bool { _, ok := legacyEducationToText[s]; return ok }},
		{"parents", "education_2", func(s string) bool { _, ok := legacyEducationToText[s]; return ok }},
		{"parents", "contact", func(s string) bool {
			_, ok := legacyContactToText[s]
			return ok || s == "Unknown" || s == "No Preference" || s == ""
		}},
		{"parents", "phone_type_1", func(s string) bool { _, ok := legacyPhoneTypeToText[s]; return ok || s == "" }},
		{"parents", "phone_type_2", func(s string) bool { _, ok := legacyPhoneTypeToText[s]; return ok || s == "" }},
		{"lab_members", "phone_type_1", func(s string) bool { _, ok := legacyPhoneTypeToText[s]; return ok || s == "" }},
		{"lab_members", "phone_type_2", func(s string) bool { _, ok := legacyPhoneTypeToText[s]; return ok || s == "" }},
		{"lab_members", "priority", func(s string) bool { _, ok := legacyPriorityToText[s]; return ok }},
		{"appointments", "schedule_status", func(s string) bool { _, ok := legacyStatusToText[s]; return ok }},
		{"appointments", "data_status", func(s string) bool { _, ok := legacyDataStatusToText[s]; return ok }},
		{"appointments", "sibling_coming", func(s string) bool {
			return s == "Unknown" || s == "Coming" || s == "Not Coming" || s == "None"
		}},
		{"experiments", "status", func(s string) bool { _, ok := legacyExperimentStatusToText[s]; return ok }},
		{"experiments", "filter_premies", func(s string) bool { return s == "Yes" || s == "No" }},
	}

	var problems []string
	for _, c := range checks {
		if err := validateEnumCoverage(ctx, legacyDB, c.table, c.column, c.mapped); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) > 0 {
		msg := "unmapped legacy enum values found (fix these mappings before importing):"
		for _, p := range problems {
			msg += "\n  - " + p
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func runImportSteps(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, importedAt time.Time) error {
	if err := validateKnownEnums(ctx, legacyDB); err != nil {
		return err
	}

	var legacyImportUserID int64
	var roleIDByName map[string]int64
	var recruitmentSourceIDs map[string]int64
	var firstGuardianByFamily map[int64]int64
	var newsletterNameToID map[string]int64
	var labIDByMember map[int64]int64
	var labIDByExperiment map[int64]int64
	var labIDByAppointment map[int64]int64
	var sitterRoleByLab map[int64]int64
	var greeterRoleByLab map[int64]int64

	steps := []struct {
		name string
		fn   func(pgx.Tx) error
	}{
		{"legacy import user", func(tx pgx.Tx) error {
			id, err := importLegacyImportUser(ctx, tx)
			legacyImportUserID = id
			return err
		}},
		{"roles", func(tx pgx.Tx) error {
			ids, err := loadRoleIDs(ctx, tx)
			roleIDByName = ids
			return err
		}},
		{"labs", func(tx pgx.Tx) error { return importLabs(ctx, legacyDB, tx, report) }},
		{"simple lookups", func(tx pgx.Tx) error { return importSimpleLookups(ctx, legacyDB, tx, report, importedAt) }},
		{"condition_values", func(tx pgx.Tx) error { return importConditionValues(ctx, legacyDB, tx, report, importedAt) }},
		{"zipcodes", func(tx pgx.Tx) error { return importZipcodes(ctx, legacyDB, tx, report, importedAt) }},
		{"tokens", func(tx pgx.Tx) error { return importTokens(ctx, legacyDB, tx, report, importedAt) }},
		{"users", func(tx pgx.Tx) error {
			ids, err := importUsers(ctx, legacyDB, tx, report, roleIDByName, importedAt)
			labIDByMember = ids
			return err
		}},
		{"experiments", func(tx pgx.Tx) error {
			ids, err := importExperiments(ctx, legacyDB, tx, report, importedAt)
			labIDByExperiment = ids
			return err
		}},
		{"experiment associations", func(tx pgx.Tx) error { return importExperimentAssocs(ctx, legacyDB, tx, report) }},
		{"sitter/greeter roles", func(tx pgx.Tx) error {
			sitterIDs, greeterIDs, err := ensureSitterAndGreeterRoles(ctx, tx, report)
			sitterRoleByLab = sitterIDs
			greeterRoleByLab = greeterIDs
			return err
		}},
		{"recruitment_sources", func(tx pgx.Tx) error {
			ids, err := importRecruitmentSources(ctx, legacyDB, tx, report)
			recruitmentSourceIDs = ids
			return err
		}},
		{"families", func(tx pgx.Tx) error {
			ids, err := importFamilies(ctx, legacyDB, tx, report)
			firstGuardianByFamily = ids
			return err
		}},
		{"children", func(tx pgx.Tx) error {
			return importChildren(ctx, legacyDB, tx, report, recruitmentSourceIDs, importedAt, legacyImportUserID)
		}},
		{"notes", func(tx pgx.Tx) error { return importFreeformNotes(ctx, legacyDB, tx, report, legacyImportUserID) }},
		{"appointments", func(tx pgx.Tx) error {
			ids, err := importAppointments(ctx, legacyDB, tx, report, labIDByExperiment)
			labIDByAppointment = ids
			return err
		}},
		{"appointment_experimenters", func(tx pgx.Tx) error {
			return importAppointmentExperimenters(ctx, legacyDB, tx, report, labIDByAppointment, sitterRoleByLab, greeterRoleByLab)
		}},
		{"lab_availability_general", func(tx pgx.Tx) error {
			return importLabAvailabilityGeneral(ctx, legacyDB, tx, report, labIDByMember)
		}},
		{"lab_availability_specific", func(tx pgx.Tx) error {
			return importLabAvailabilitySpecific(ctx, legacyDB, tx, report, labIDByMember)
		}},
		{"schedule_blockings", func(tx pgx.Tx) error { return importScheduleBlockings(ctx, legacyDB, tx, report) }},
		{"newsletters", func(tx pgx.Tx) error {
			ids, err := importNewsletters(ctx, legacyDB, tx, report, importedAt)
			newsletterNameToID = ids
			return err
		}},
		{"newsletters_parents", func(tx pgx.Tx) error {
			return importNewsletterSent(ctx, legacyDB, tx, report, newsletterNameToID, firstGuardianByFamily)
		}},
	}

	for _, step := range steps {
		started := time.Now()
		slog.Info("importing", "step", step.name)
		if err := step.fn(tx); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
		slog.Info("done", "step", step.name, "elapsed", time.Since(started))
	}
	return nil
}

// loadRoleIDs reads the fixed staff/coordinator/admin roles seeded by
// the core schema migration -- importUsers needs their ids to build
// lab_memberships rows.
func loadRoleIDs(ctx context.Context, tx pgx.Tx) (map[string]int64, error) {
	rows, err := tx.Query(ctx, "select id, name from roles")
	if err != nil {
		return nil, fmt.Errorf("load roles: %w", err)
	}
	defer rows.Close()
	ids := map[string]int64{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		ids[name] = id
	}
	return ids, rows.Err()
}
