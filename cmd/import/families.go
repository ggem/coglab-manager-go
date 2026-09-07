package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var legacyContactToText = map[string]string{
	"Home Phone":   "home_phone",
	"Work Phone":   "work_phone",
	"Mobile Phone": "mobile_phone",
	"Fax":          "fax",
	"Email":        "email",
	"Snail Mail":   "snail_mail",
	// "Unknown", "No Preference" (a live-only value, not in the
	// create-database reference script), and "" all fall through to nil
	// below -- families.preferred_contact_method is nullable precisely
	// for "not specified" cases like these.
}

func mapContactMethod(legacy string) *string {
	if v, ok := legacyContactToText[legacy]; ok {
		return &v
	}
	return nil
}

// legacyEducationToText: the live database's education_1/2 enum has
// drifted from the create-database reference script -- "Degree From a
// 4 Year College or Higher" was renamed to drop "or Higher", and a new
// "Graduate Degree" value was added with no category of its own in
// this schema's education check constraint. Both fold into
// degree_from_4yr_college_or_higher, whose own name already reads as
// an inclusive "this or higher" bucket -- validateEnumCoverage (see
// runImportSteps) catches any *future* drift before it can silently
// turn into thousands of per-row failures like these did.
var legacyEducationToText = map[string]string{
	"Unknown":                                "unknown",
	"Without High School Diploma":            "without_high_school_diploma",
	"HS Grad, No College":                    "hs_grad_no_college",
	"HS Grad, Some College":                  "hs_grad_some_college",
	"Degree From a 4 Year College or Higher": "degree_from_4yr_college_or_higher",
	"Degree From a 4 Year College":           "degree_from_4yr_college_or_higher",
	"Graduate Degree":                        "degree_from_4yr_college_or_higher",
	"Left Blank":                             "left_blank",
}

func mapEducation(legacy string) (string, error) {
	if v, ok := legacyEducationToText[legacy]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unrecognized education %q", legacy)
}

// legacyPhoneTypeToText maps the shared phone_type enum used by both
// parents and lab_members (' (Home)', ' (Work)', ... -- note the
// legacy values' leading space, an artifact of how the enum reads in a
// sentence like "555-1234 (Home)" in the legacy UI). "" is legacy's
// "not specified", mapped to nil like mapContactMethod above.
var legacyPhoneTypeToText = map[string]string{
	" (Home)":         "home",
	" (Work)":         "work",
	" (Mobile)":       "mobile",
	" (Fax)":          "fax",
	" (Pager)":        "pager",
	" (Disconnected)": "disconnected",
	" (Other)":        "other",
}

func mapPhoneType(legacy string) *string {
	if v, ok := legacyPhoneTypeToText[legacy]; ok {
		return &v
	}
	return nil
}

// importFamilies imports parents as families + up to two guardians per
// row, from the _1/_2 field pairs -- a slot is skipped entirely when
// both its name fields are empty, legacy's convention for "no second
// parent" (see M10 planning memory). families.id reuses parents_id
// directly (a clean 1:1 mapping); guardians has no legacy id of its own
// to preserve, so slot 1 becomes parents_id*10+1 and slot 2 parents_id*10+2
// -- deterministic and collision-free (parents_id is a MySQL mediumint,
// comfortably within bigint range), which keeps a second import run
// idempotent (failing on a real PK conflict) rather than silently
// duplicating guardians the way a plain auto-generated id would.
func importFamilies(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report) (firstGuardianByFamily map[int64]int64, err error) {
	rows, err := queryLegacy(ctx, legacyDB, "select * from parents")
	if err != nil {
		return nil, err
	}
	firstGuardianByFamily = make(map[int64]int64, len(rows))
	for _, row := range rows {
		id := row.int64("parents_id")

		familyFields := map[string]any{
			"id":                       id,
			"address":                  row.str("address"),
			"city":                     row.str("city"),
			"state":                    row.str("state"),
			"zip":                      row.str("zip"),
			"preferred_contact_method": mapContactMethod(row.str("contact")),
		}

		type guardianSlot struct {
			firstName, lastName, education, occupation, phone, phoneType, email string
		}
		slots := []guardianSlot{
			{row.str("first_name_1"), row.str("last_name_1"), row.str("education_1"), row.str("occupation_1"), row.str("phone_number_1"), row.str("phone_type_1"), row.str("email_address")},
			{row.str("first_name_2"), row.str("last_name_2"), row.str("education_2"), row.str("occupation_2"), row.str("phone_number_2"), row.str("phone_type_2"), ""},
		}

		var firstGuardianID int64
		txErr := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			if err := insertRow(ctx, sp, "families", familyFields); err != nil {
				return err
			}
			for i, slot := range slots {
				if slot.firstName == "" && slot.lastName == "" {
					continue
				}
				education, err := mapEducation(slot.education)
				if err != nil {
					return fmt.Errorf("guardian %d: %w", i+1, err)
				}
				guardianID := id*10 + int64(i+1)
				err = insertRow(ctx, sp, "guardians", map[string]any{
					"id":           guardianID,
					"family_id":    id,
					"first_name":   slot.firstName,
					"last_name":    slot.lastName,
					"education":    education,
					"occupation":   slot.occupation,
					"phone_number": slot.phone,
					"phone_type":   mapPhoneType(slot.phoneType),
					"email":        slot.email,
				})
				if err != nil {
					return fmt.Errorf("guardian %d: %w", i+1, err)
				}
				if firstGuardianID == 0 {
					firstGuardianID = guardianID
				}
			}
			return nil
		})
		if txErr != nil {
			report.Error("families", id, txErr)
			continue
		}
		report.Ok("families")
		if firstGuardianID != 0 {
			firstGuardianByFamily[id] = firstGuardianID
		}
	}
	if err := bumpSequence(ctx, tx, "families"); err != nil {
		return nil, err
	}
	if err := bumpSequence(ctx, tx, "guardians"); err != nil {
		return nil, err
	}
	return firstGuardianByFamily, nil
}
