package main

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// deactivatedAt maps legacy's record_status enum ('Ok'/'Deleted') onto
// this app's deactivated_at convention. Legacy never recorded *when* a
// row was deleted, so an imported deactivated row is timestamped "now"
// (the import moment) rather than a fabricated historical date.
func deactivatedAt(recordStatus string, importedAt time.Time) *time.Time {
	if recordStatus == "Deleted" {
		return &importedAt
	}
	return nil
}

// simpleLookupSpec describes one of the plain lab-scoped lookup tables
// (conditions, experiment_types, protocols, grants, roles) that all
// share the exact legacy shape (id, name, lab_id, record_status) and
// target shape (id, lab_id, name, deactivated_at). extraFields lets a
// handful of tables (equipment's quantity, protocols' description) add
// their one or two extra columns without a whole separate function.
type simpleLookupSpec struct {
	legacyTable string
	legacyIDCol string
	targetTable string
	extraFields func(legacyRow) map[string]any
}

var simpleLookups = []simpleLookupSpec{
	{legacyTable: "conditions", legacyIDCol: "condition_id", targetTable: "conditions"},
	{legacyTable: "experiment_types", legacyIDCol: "experiment_type_id", targetTable: "experiment_types"},
	{legacyTable: "grants", legacyIDCol: "grant_id", targetTable: "grants"},
	{legacyTable: "roles", legacyIDCol: "role_id", targetTable: "experiment_roles"},
	// protocols has a legacy "description" column, but the target
	// protocols table (see the reporting-schema migration) has no such
	// column -- dropped deliberately during that migration, not an
	// oversight here, so it's a plain simpleLookupSpec with no
	// extraFields like grants/experiment_types above.
	{legacyTable: "protocols", legacyIDCol: "protocol_id", targetTable: "protocols"},
	{
		legacyTable: "equipment", legacyIDCol: "equipment_id", targetTable: "equipment",
		extraFields: func(r legacyRow) map[string]any { return map[string]any{"quantity": r.int64("quantity")} },
	},
}

func importSimpleLookups(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, importedAt time.Time) error {
	for _, spec := range simpleLookups {
		rows, err := queryLegacy(ctx, legacyDB, "select * from "+spec.legacyTable)
		if err != nil {
			return err
		}
		for _, row := range rows {
			id := row.int64(spec.legacyIDCol)
			fields := map[string]any{
				"id":             id,
				"lab_id":         row.int64("lab_id"),
				"name":           row.str("name"),
				"deactivated_at": deactivatedAt(row.str("record_status"), importedAt),
			}
			if spec.extraFields != nil {
				for k, v := range spec.extraFields(row) {
					fields[k] = v
				}
			}
			err := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
				return insertRow(ctx, sp, spec.targetTable, fields)
			})
			if err != nil {
				report.Error(spec.targetTable, id, err)
				continue
			}
			report.Ok(spec.targetTable)
		}
		if err := bumpSequence(ctx, tx, spec.targetTable); err != nil {
			return err
		}
	}
	return nil
}

// importConditionValues depends on conditions already being imported
// (condition_id FK), so it's separate from the simpleLookups loop above
// rather than folded in with an artificial ordering dependency.
func importConditionValues(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, importedAt time.Time) error {
	rows, err := queryLegacy(ctx, legacyDB, "select * from condition_values")
	if err != nil {
		return err
	}
	for _, row := range rows {
		id := row.int64("condition_value_id")
		fields := map[string]any{
			"id":             id,
			"condition_id":   row.int64("condition_id"),
			"name":           row.str("name"),
			"deactivated_at": deactivatedAt(row.str("record_status"), importedAt),
		}
		err := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			return insertRow(ctx, sp, "condition_values", fields)
		})
		if err != nil {
			report.Error("condition_values", id, err)
			continue
		}
		report.Ok("condition_values")
	}
	return bumpSequence(ctx, tx, "condition_values")
}

// importZipcodes: legacy's priority is a plain integer tier (tinyint);
// this app's zipcodes.priority is free-form text (no fixed vocabulary),
// so the integer is preserved as its decimal string rather than mapped
// onto any label.
func importZipcodes(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, importedAt time.Time) error {
	rows, err := queryLegacy(ctx, legacyDB, "select * from zipcodes")
	if err != nil {
		return err
	}
	for _, row := range rows {
		id := row.int64("zipcode_id")
		fields := map[string]any{
			"id":             id,
			"lab_id":         row.int64("lab_id"),
			"zip_code":       row.str("name"),
			"priority":       formatInt64(row.int64("priority")),
			"deactivated_at": deactivatedAt(row.str("record_status"), importedAt),
		}
		err := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			return insertRow(ctx, sp, "zipcodes", fields)
		})
		if err != nil {
			report.Error("zipcodes", id, err)
			continue
		}
		report.Ok("zipcodes")
	}
	return bumpSequence(ctx, tx, "zipcodes")
}

func importLabs(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report) error {
	rows, err := queryLegacy(ctx, legacyDB, "select * from labs")
	if err != nil {
		return err
	}
	for _, row := range rows {
		id := row.int64("lab_id")
		fields := map[string]any{
			"id":         id,
			"name":       row.str("name"),
			"short_name": row.str("short_name"),
		}
		err := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			return insertRow(ctx, sp, "labs", fields)
		})
		if err != nil {
			report.Error("labs", id, err)
			continue
		}
		report.Ok("labs")
	}
	return bumpSequence(ctx, tx, "labs")
}

// importTokens: end-of-appointment participant gift tracking (see the
// M10 planning memory) -- straightforward lab-scoped lookup, same shape
// as the simpleLookups above but kept separate since it has no legacy
// "id" column named consistently with the others' <name>_id pattern
// worth generalizing over (token_id).
func importTokens(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, importedAt time.Time) error {
	rows, err := queryLegacy(ctx, legacyDB, "select * from tokens")
	if err != nil {
		return err
	}
	for _, row := range rows {
		id := row.int64("token_id")
		fields := map[string]any{
			"id":             id,
			"lab_id":         row.int64("lab_id"),
			"name":           row.str("name"),
			"deactivated_at": deactivatedAt(row.str("record_status"), importedAt),
		}
		err := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			return insertRow(ctx, sp, "tokens", fields)
		})
		if err != nil {
			report.Error("tokens", id, err)
			continue
		}
		report.Ok("tokens")
	}
	return bumpSequence(ctx, tx, "tokens")
}

func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}
