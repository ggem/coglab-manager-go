package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// roleForAuthLevel maps legacy's numeric lab_members.auth_level onto
// this app's 3-tier roles.name, confirmed from the Scheme source's own
// menu-gating thresholds (org.ggem.kids.participants.ss's menu-entries
// table): level 1 is baseline day-to-day access (schedule, search,
// confirm, release, reports); level 3 additionally allows adding
// experiments; level 4 is full admin (add lab members, all Configure/
// DBConfig sections). Level 0 (effectively no access beyond logout) and
// level 2 (never used as a gating threshold in the source, but a real
// value some rows might have) both fold into staff -- the lowest real
// tier -- rather than a fourth, invented category.
func roleForAuthLevel(authLevel int64) string {
	switch {
	case authLevel >= 4:
		return "admin"
	case authLevel >= 3:
		return "coordinator"
	default:
		return "staff"
	}
}

// legacyPriorityToText maps lab_members.priority's 6-value legacy enum
// (declaration order preserved) onto lab_memberships.priority's text
// values -- see the M10 planning memory for why this field matters
// (real scheduling preference, not descriptive metadata).
var legacyPriorityToText = map[string]string{
	"Undergrad/Community Volunteer w/o independent project": "undergrad_no_project",
	"Undergrad with independent project":                    "undergrad_with_project",
	"Lab coordinator":                                       "lab_coordinator",
	"Graduate Student":                                      "graduate_student",
	"Postdoc":                                               "postdoc",
	"Lab Director":                                          "lab_director",
}

func mapPriority(legacy string) (string, error) {
	if v, ok := legacyPriorityToText[legacy]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unrecognized priority %q", legacy)
}

// importUsers imports lab_members as users + lab_memberships. A legacy
// lab_member row is really "one person, one lab, one role_id (fixed to
// a single system role, not this app's per-lab roles table)" -- this
// app's users are lab-independent, so member_id becomes the user id and
// a matching lab_memberships row carries the lab-scoped auth
// level/priority. year (descriptive only, not used for access control
// or scheduling anywhere in the source) is intentionally dropped.
func importUsers(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, roleIDByName map[string]int64, importedAt time.Time) (labIDByMember map[int64]int64, err error) {
	rows, err := queryLegacy(ctx, legacyDB, "select * from lab_members")
	if err != nil {
		return nil, err
	}
	labIDByMember = make(map[int64]int64, len(rows))
	for _, row := range rows {
		id := row.int64("member_id")
		labIDByMember[id] = row.int64("lab_id")

		priority, err := mapPriority(row.str("priority"))
		if err != nil {
			report.Error("lab_memberships", id, err)
			continue
		}
		roleName := roleForAuthLevel(row.int64("auth_level"))
		roleID, ok := roleIDByName[roleName]
		if !ok {
			report.Error("lab_memberships", id, fmt.Errorf("no roles.id for role %q", roleName))
			continue
		}

		deactivated := deactivatedAt(row.str("record_status"), importedAt)
		userFields := map[string]any{
			"id":                id,
			"email":             row.str("email_address"),
			"first_name":        row.str("first_name"),
			"last_name":         row.str("last_name"),
			"is_platform_admin": false,
			"deactivated_at":    deactivated,
		}
		if created := row.nullDate("datetime_created"); created != nil {
			userFields["created_at"] = *created
		}
		membershipFields := map[string]any{
			"id":       id,
			"user_id":  id,
			"lab_id":   row.int64("lab_id"),
			"role_id":  roleID,
			"priority": priority,
		}
		err = withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			if err := insertRow(ctx, sp, "users", userFields); err != nil {
				return err
			}
			return insertRow(ctx, sp, "lab_memberships", membershipFields)
		})
		if err != nil {
			report.Error("users", id, err)
			continue
		}
		report.Ok("users")
		report.Ok("lab_memberships")
	}
	if err := bumpSequence(ctx, tx, "users"); err != nil {
		return nil, err
	}
	if err := bumpSequence(ctx, tx, "lab_memberships"); err != nil {
		return nil, err
	}
	return labIDByMember, nil
}
