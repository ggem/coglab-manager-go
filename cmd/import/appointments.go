package main

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
)

// legacyStatusToText: the live database's schedule_status enum has one
// value the create-database reference script doesn't document at all
// -- lowercase "rescheduled", added at some point directly against the
// live schema. Mapped to to_be_scheduled (needs a new slot found),
// the closest existing status -- validateEnumCoverage (see
// runImportSteps) would have caught this immediately instead of
// producing tens of thousands of identical per-row errors, which is
// exactly how it was actually found.
var legacyStatusToText = map[string]string{
	"To be scheduled": "to_be_scheduled",
	"Pending":         "pending",
	"Arrived":         "arrived",
	"No Show":         "no_show",
	"Canceled":        "canceled",
	"Problem":         "problem",
	"Released":        "released",
	"rescheduled":     "to_be_scheduled",
}

func mapStatus(legacy string) (string, error) {
	if v, ok := legacyStatusToText[legacy]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unrecognized schedule_status %q", legacy)
}

// legacyDataStatusToText: same live-vs-reference drift as
// legacyStatusToText above -- "All yes/all no" (a data-quality flag,
// presumably meaning every trial went the same way) has no category of
// its own here, so it folds into "other" rather than misrepresenting
// it as "ok" or inventing a new column value.
var legacyDataStatusToText = map[string]string{
	"Ok":                 "ok",
	"Fuss":               "fuss",
	"Experimental Error": "experimental_error",
	"Equipment Failure":  "equipment_failure",
	"Other":              "other",
	"Extra":              "extra",
	"No Data":            "no_data",
	"All yes/all no":     "other",
}

func mapDataStatus(legacy string) (string, error) {
	if v, ok := legacyDataStatusToText[legacy]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unrecognized data_status %q", legacy)
}

// mapSiblingComing maps legacy's 4-value sibling_coming enum onto this
// app's 3-value one -- 'None' folds into 'not_coming', matching the
// project decision that legacy's 3rd/4th states were never meaningfully
// distinguished in practice (see the sibling_coming_semantics memory).
func mapSiblingComing(legacy string) (string, error) {
	switch legacy {
	case "Unknown":
		return "unknown", nil
	case "Coming":
		return "coming", nil
	case "Not Coming", "None":
		return "not_coming", nil
	default:
		return "", fmt.Errorf("unrecognized sibling_coming %q", legacy)
	}
}

func importAppointments(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, labIDByExperiment map[int64]int64) (labIDByAppointment map[int64]int64, err error) {
	rows, err := queryLegacy(ctx, legacyDB, "select * from appointments")
	if err != nil {
		return nil, err
	}
	labIDByAppointment = make(map[int64]int64, len(rows))
	for _, row := range rows {
		id := row.int64("appointment_id")
		experimentID := row.int64("experiment_id")
		if labID, ok := labIDByExperiment[experimentID]; ok {
			labIDByAppointment[id] = labID
		}

		status, err := mapStatus(row.str("schedule_status"))
		if err != nil {
			report.Error("appointments", id, err)
			continue
		}
		dataStatus, err := mapDataStatus(row.str("data_status"))
		if err != nil {
			report.Error("appointments", id, err)
			continue
		}
		siblingComing, err := mapSiblingComing(row.str("sibling_coming"))
		if err != nil {
			report.Error("appointments", id, err)
			continue
		}

		fields := map[string]any{
			"id":                   id,
			"experiment_id":        row.int64("experiment_id"),
			"child_id":             row.int64("child_id"),
			"session":              row.int64("session"),
			"age_range_min_months": row.nullFloat64("age_range_min"),
			"age_range_max_months": row.nullFloat64("age_range_max"),
			"sibling_coming":       siblingComing,
			"schedule_date":        row.nullDate("schedule_date"),
			"status":               status,
			"data_status":          dataStatus,
			"type_of_car":          row.str("type_of_car"),
			"participant_number":   row.str("participant_number"),
			"token_id":             row.nullInt64("token_id"),
			"schedule_time_start":  row.nullTimeStr("schedule_time_start"),
			"schedule_time_end":    row.nullTimeStr("schedule_time_end"),
		}

		err = withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			return insertRow(ctx, sp, "appointments", fields)
		})
		if err != nil {
			report.Error("appointments", id, err)
			continue
		}
		report.Ok("appointments")
	}
	if err := bumpSequence(ctx, tx, "appointments"); err != nil {
		return nil, err
	}
	return labIDByAppointment, nil
}

// legacyGreeterRoleID and legacySitterRoleID are two well-known legacy
// role_ids (confirmed live: "Greeter" and "Sitter" in the roles table,
// both with lab_id = 0) that aren't real per-lab trained roles at all --
// see ensureSitterAndGreeterRoles.
const (
	legacyGreeterRoleID = 1
	legacySitterRoleID  = 2
)

// ensureSitterAndGreeterRoles creates one "Sitter" (is_sitter_role =
// true, matching the FM5 feature's per-lab designation) and one
// "Greeter" experiment_role per lab. Neither has a real per-lab
// equivalent in legacy: every lab shared the same two global (lab_id=0)
// role rows instead. Confirmed with a lab director that "greeter with
// no other role" is a real, intentional assignment (freeing the sole
// experimenter from having to leave stimuli unattended to answer the
// door), not legacy data corruption -- so, like Sitter, Greeter needs a
// real per-lab home rather than being dropped. Synthetic ids
// (800000000 + labID*10 + 1/2) stay well clear of any real legacy id
// range, matching the same collision-avoidance approach as the Legacy
// Import pseudo-user in notes.go.
func ensureSitterAndGreeterRoles(ctx context.Context, tx pgx.Tx, report *Report) (sitterRoleByLab, greeterRoleByLab map[int64]int64, err error) {
	rows, err := tx.Query(ctx, "select id from labs order by id")
	if err != nil {
		return nil, nil, fmt.Errorf("load lab ids: %w", err)
	}
	defer rows.Close()
	var labIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, nil, err
		}
		labIDs = append(labIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	sitterRoleByLab = make(map[int64]int64, len(labIDs))
	greeterRoleByLab = make(map[int64]int64, len(labIDs))
	for _, labID := range labIDs {
		sitterID := 800000000 + labID*10 + 1
		greeterID := 800000000 + labID*10 + 2
		err := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			if err := insertRow(ctx, sp, "experiment_roles", map[string]any{
				"id": sitterID, "lab_id": labID, "name": "Sitter", "is_sitter_role": true,
			}); err != nil {
				return err
			}
			return insertRow(ctx, sp, "experiment_roles", map[string]any{
				"id": greeterID, "lab_id": labID, "name": "Greeter",
			})
		})
		if err != nil {
			report.Error("experiment_roles", labID, err)
			continue
		}
		sitterRoleByLab[labID] = sitterID
		greeterRoleByLab[labID] = greeterID
	}
	return sitterRoleByLab, greeterRoleByLab, nil
}

// splitGreeter separates a (appointment_id, member_id) group's role_ids
// into whether it includes the Greeter sentinel and everything else,
// deduplicating otherRoles -- legacy sometimes has the exact same
// (appointment, member, role) triple recorded twice, which isn't a real
// conflict (two different roles), just a redundant duplicate row.
func splitGreeter(roleIDs []int64) (hasGreeter bool, otherRoles []int64) {
	for _, r := range roleIDs {
		switch {
		case r == legacyGreeterRoleID:
			hasGreeter = true
		case !slices.Contains(otherRoles, r):
			otherRoles = append(otherRoles, r)
		}
	}
	return hasGreeter, otherRoles
}

// resolveExperimenterRole implements the merge algorithm described on
// importAppointmentExperimenters: exactly one non-greeter role passes
// through as-is (remapped to the lab's Sitter role if it's the Sitter
// sentinel); zero non-greeter roles (a greeter-only assignment) resolve
// to the lab's Greeter role; more than one non-greeter role is a
// genuine conflict this schema's one-row-per-(appointment,user)
// constraint can't represent, so it's reported rather than guessed at.
func resolveExperimenterRole(memberID int64, otherRoles []int64, labID int64, hasLab bool, sitterRoleByLab, greeterRoleByLab map[int64]int64) (int64, error) {
	switch {
	case len(otherRoles) > 1:
		return 0, fmt.Errorf("member %d has more than one non-greeter role on this appointment: %v", memberID, otherRoles)
	case len(otherRoles) == 1 && otherRoles[0] != legacySitterRoleID:
		return otherRoles[0], nil
	}
	if !hasLab {
		return 0, fmt.Errorf("no known lab for this appointment")
	}
	byLab := greeterRoleByLab
	roleName := "Greeter"
	if len(otherRoles) == 1 { // otherRoles[0] == legacySitterRoleID here
		byLab, roleName = sitterRoleByLab, "Sitter"
	}
	roleID, ok := byLab[labID]
	if !ok {
		return 0, fmt.Errorf("no %s role created for lab %d", roleName, labID)
	}
	return roleID, nil
}

// importAppointmentExperimenters resolves legacy's global Greeter/
// Sitter role rows before inserting, since neither maps onto a real
// per-lab experiment_role directly:
//
//   - Sitter always resolves to that appointment's lab's per-lab Sitter
//     role from ensureSitterAndGreeterRoles.
//   - Greeter merges onto whichever other real role the same member
//     already holds on the same appointment (setting is_greeter=true
//     there) when one exists -- confirmed live: 87% of the time it
//     does. When it doesn't (confirmed with a lab director as a real,
//     intentional assignment -- freeing the sole experimenter from
//     leaving stimuli unattended -- not corrupt data), it resolves to
//     that lab's per-lab Greeter role instead, still with
//     is_greeter=true.
//   - Any other role_id imports directly as-is (it's already a real,
//     specific experiment_role from the roles table).
//
// Grouped by (appointment_id, member_id) via a stable ORDER BY rather
// than a SQL GROUP BY, since resolving a group needs per-role-id
// branching logic more naturally expressed in Go than SQL.
func importAppointmentExperimenters(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, labIDByAppointment, sitterRoleByLab, greeterRoleByLab map[int64]int64) error {
	rows, err := queryLegacy(ctx, legacyDB, "select * from appointment_experimenters order by appointment_id, member_id")
	if err != nil {
		return err
	}

	type key struct{ appointmentID, memberID int64 }
	groups := map[key][]int64{}
	var order []key
	for _, row := range rows {
		k := key{row.int64("appointment_id"), row.int64("member_id")}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], row.int64("role_id"))
	}

	for _, k := range order {
		hasGreeter, otherRoles := splitGreeter(groups[k])

		labID, hasLab := labIDByAppointment[k.appointmentID]
		finalRoleID, err := resolveExperimenterRole(k.memberID, otherRoles, labID, hasLab, sitterRoleByLab, greeterRoleByLab)
		if err != nil {
			report.Error("appointment_experimenters", k.appointmentID, err)
			continue
		}

		fields := map[string]any{
			"appointment_id":     k.appointmentID,
			"user_id":            k.memberID,
			"experiment_role_id": finalRoleID,
			"is_greeter":         hasGreeter,
		}
		err = withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			return insertAssoc(ctx, sp, "appointment_experimenters", fields)
		})
		if err != nil {
			report.Error("appointment_experimenters", k.appointmentID, err)
			continue
		}
		report.Ok("appointment_experimenters")
	}
	return nil
}
