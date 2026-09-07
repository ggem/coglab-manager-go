package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var legacyExperimentStatusToText = map[string]string{
	"Not Run": "not_run",
	"Pilot":   "pilot",
	"Run":     "run",
}

func mapExperimentStatus(legacy string) (string, error) {
	if v, ok := legacyExperimentStatusToText[legacy]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unrecognized experiment status %q", legacy)
}

func importExperiments(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, importedAt time.Time) (labIDByExperiment map[int64]int64, err error) {
	rows, err := queryLegacy(ctx, legacyDB, "select * from experiments")
	if err != nil {
		return nil, err
	}
	labIDByExperiment = make(map[int64]int64, len(rows))
	for _, row := range rows {
		id := row.int64("experiment_id")
		labIDByExperiment[id] = row.int64("lab_id")

		status, err := mapExperimentStatus(row.str("status"))
		if err != nil {
			report.Error("experiments", id, err)
			continue
		}

		fields := map[string]any{
			"id":                   id,
			"lab_id":               row.int64("lab_id"),
			"name":                 row.str("name"),
			"description":          row.str("description"),
			"sessions":             row.int64("sessions"),
			"age_range_min_months": row.nullFloat64("age_range_min"),
			"age_range_max_months": row.nullFloat64("age_range_max"),
			"start_date":           row.nullDate("start_date"),
			"end_date":             row.nullDate("end_date"),
			"status":               status,
			"duration_minutes":     row.int64("duration"),
			"filter_premies":       row.str("filter_premies") == "Yes",
			"filter_min_languages": row.int64("filter_num_langs"),
			"filter_languages":     mapLanguages(row.str("filter_languages")),
			"experiment_type_id":   row.nullInt64("experiment_type_id"),
			"protocol_id":          row.nullInt64("protocol_id"),
			"deactivated_at":       deactivatedAt(row.str("record_status"), importedAt),
		}

		err = withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			return insertRow(ctx, sp, "experiments", fields)
		})
		if err != nil {
			report.Error("experiments", id, err)
			continue
		}
		report.Ok("experiments")
	}
	if err := bumpSequence(ctx, tx, "experiments"); err != nil {
		return nil, err
	}
	return labIDByExperiment, nil
}

// experimentAssocSpec describes one of the plain experiment_* join
// tables that share the shape (experiment_id, <other>_id) with no
// transformation needed beyond a straight column copy/rename.
type experimentAssocSpec struct {
	legacyTable string
	targetTable string
	otherCol    string // legacy column name for the non-experiment side
	targetOther string // target column name for the non-experiment side
}

var experimentAssocs = []experimentAssocSpec{
	{legacyTable: "experiment_conditions", targetTable: "experiment_conditions", otherCol: "condition_id", targetOther: "condition_id"},
	{legacyTable: "experiment_equipment_requirements", targetTable: "experiment_equipment_requirements", otherCol: "equipment_id", targetOther: "equipment_id"},
	{legacyTable: "experiment_training_requirements", targetTable: "experiment_training_requirements", otherCol: "role_id", targetOther: "experiment_role_id"},
	{legacyTable: "experiment_grants", targetTable: "experiment_grants", otherCol: "grant_id", targetOther: "grant_id"},
	{legacyTable: "principal_investigators", targetTable: "experiment_principal_investigators", otherCol: "member_id", targetOther: "user_id"},
	// exp_id/experiment_id's direction in legacy's own two exclusion/
	// inclusion tables isn't documented anywhere in the schema comments
	// or the Scheme source's table definitions -- read as "for
	// experiment_id, exclude/include exp_id", the more natural English
	// reading of the column pairing. Worth double-checking against the
	// live legacy app's actual exclusion-list UI before this feature
	// gets its own frontend, since nothing reads these tables yet.
	{legacyTable: "experiment_experiments_to_exclude", targetTable: "experiment_exclusions", otherCol: "exp_id", targetOther: "excluded_experiment_id"},
	{legacyTable: "experiment_experiments_to_include", targetTable: "experiment_inclusions", otherCol: "exp_id", targetOther: "included_experiment_id"},
}

func importExperimentAssocs(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report) error {
	for _, spec := range experimentAssocs {
		rows, err := queryLegacy(ctx, legacyDB, "select * from "+spec.legacyTable)
		if err != nil {
			return err
		}
		for _, row := range rows {
			experimentID := row.int64("experiment_id")
			fields := map[string]any{
				"experiment_id":  experimentID,
				spec.targetOther: row.int64(spec.otherCol),
			}
			err := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
				return insertAssoc(ctx, sp, spec.targetTable, fields)
			})
			if err != nil {
				report.Error(spec.targetTable, experimentID, err)
				continue
			}
			report.Ok(spec.targetTable)
		}
	}
	return nil
}
