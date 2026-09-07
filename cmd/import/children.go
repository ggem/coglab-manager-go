package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// importRecruitmentSources seeds recruitment_sources from the *live*
// children.source enum definition (read via legacyEnumValues, not a
// hardcoded snapshot) and returns a name -> id lookup for
// importChildren. This table's schema has drifted substantially from
// the create-database reference script over ~20 years of direct ALTER
// TABLE statements -- confirmed live at 61 values, not the 26
// documented there -- so deriving ids from whatever the enum actually
// declares, in its declared order, is what keeps a rerun's ids stable
// (idempotent) without also requiring this list to be hand-maintained
// forever in lockstep with a schema nothing else here tracks.
func importRecruitmentSources(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report) (map[string]int64, error) {
	sourceValues, err := legacyEnumValues(ctx, legacyDB, "children", "source")
	if err != nil {
		return nil, err
	}
	ids := make(map[string]int64, len(sourceValues))
	for i, name := range sourceValues {
		id := int64(i + 1)
		fields := map[string]any{"id": id, "name": name, "active": true}
		err := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			return insertRow(ctx, sp, "recruitment_sources", fields)
		})
		if err != nil {
			report.Error("recruitment_sources", id, err)
			continue
		}
		report.Ok("recruitment_sources")
		ids[name] = id
	}
	if err := bumpSequence(ctx, tx, "recruitment_sources"); err != nil {
		return nil, err
	}
	return ids, nil
}

var legacyRaceToCategory = map[string]string{
	"American Indian/Alaska Native": "american_indian_or_alaska_native",
	"Asian":                         "asian",
	"Native Hawaiian or Other Pacific Islander": "native_hawaiian_or_pacific_islander",
	"Black or African American":                 "black_or_african_american",
	"White":                                     "white",
	// "Unknown or Not Reported" (race) and "No other race"/"Unknown or
	// Not Reported" (other_race) all contribute nothing -- absence in
	// race_ethnicity[] is this schema's own "not reported" convention.
}

// mapRaceEthnicity merges legacy's separate ethnicity/race/other_race
// columns into this app's single race_ethnicity[] "select all that
// apply" array -- the reverse of the mapping nih_report.go already does
// for the NIH export. A category appears only for an explicit "this
// applies" answer; "Unknown"/"Not Hispanic or Latino"/"No other race"
// all contribute nothing, matching the schema's convention that an
// empty array means "not reported" (see the participants-schema
// migration's own comment) -- this does mean a child explicitly marked
// "Not Hispanic or Latino" with an unreported race ends up
// indistinguishable from "never asked," an inherent lossiness the
// schema already accepted for newly-entered data, not something new
// import introduces.
func mapRaceEthnicity(ethnicity, race, otherRace string) []string {
	// Explicitly non-nil: race_ethnicity is "not null default '{}'", and
	// pgx encodes a nil []string as SQL NULL rather than an empty array.
	categories := []string{}
	add := func(c string) {
		for _, existing := range categories {
			if existing == c {
				return
			}
		}
		categories = append(categories, c)
	}
	if ethnicity == "Hispanic or Latino" {
		add("hispanic_or_latino")
	}
	if c, ok := legacyRaceToCategory[race]; ok {
		add(c)
	}
	if c, ok := legacyRaceToCategory[otherRace]; ok {
		add(c)
	}
	return categories
}

func mapSex(gender string) string {
	switch gender {
	case "Male":
		return "male"
	case "Female":
		return "female"
	default:
		return "unknown"
	}
}

// triStateBool maps legacy's Unknown/Yes/No enums (birth_comp, twin)
// onto a nullable boolean -- null already means "unknown" in SQL, so
// there's no need for this schema's own three-state type.
func triStateBool(legacy string) *bool {
	switch legacy {
	case "Yes":
		v := true
		return &v
	case "No":
		v := false
		return &v
	default:
		return nil
	}
}

// triStatePremie maps children.premie's own, differently-labeled
// tri-state enum (Unknown/Premie/Full Term -- not Yes/No like
// birth_comp/twin) onto the same nullable boolean shape.
func triStatePremie(legacy string) *bool {
	switch legacy {
	case "Premie":
		v := true
		return &v
	case "Full Term":
		v := false
		return &v
	default:
		return nil
	}
}

func mapLanguages(legacySet string) []string {
	if legacySet == "" {
		return []string{}
	}
	parts := strings.Split(legacySet, ",")
	langs := make([]string, len(parts))
	for i, p := range parts {
		langs[i] = strings.ToLower(strings.TrimSpace(p))
	}
	return langs
}

var legacyResponseToText = map[string]string{
	"Unknown":    "unknown",
	"Email":      "email",
	"Snail Mail": "snail_mail",
	"Phone":      "phone",
	"Web Page":   "web_page",
}

func mapResponse(legacy string) (string, error) {
	if v, ok := legacyResponseToText[legacy]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unrecognized response %q", legacy)
}

func importChildren(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, recruitmentSourceIDs map[string]int64, importedAt time.Time, legacyImportUserID int64) error {
	rows, err := queryLegacy(ctx, legacyDB, "select * from children")
	if err != nil {
		return err
	}
	for _, row := range rows {
		id := row.int64("child_id")

		response, err := mapResponse(row.str("response"))
		if err != nil {
			report.Error("children", id, err)
			continue
		}
		sourceName := row.str("source")
		sourceID, ok := recruitmentSourceIDs[sourceName]
		if !ok {
			report.Error("children", id, fmt.Errorf("unrecognized source %q", sourceName))
			continue
		}
		// lab_member_creator = 0 is legacy's "no real value" sentinel
		// (confirmed live: ~44% of all children), not a real staff
		// member -- attributed to the same Legacy Import pseudo-user
		// notes.go already uses, per the M10 planning decision, rather
		// than dropping nearly half of all participant records.
		creatorID := row.int64("lab_member_creator")
		if creatorID == 0 {
			creatorID = legacyImportUserID
		}

		fields := map[string]any{
			"id":                       id,
			"family_id":                row.int64("parents_id"),
			"first_name":               row.str("first_name"),
			"last_name":                row.str("last_name"),
			"sex":                      mapSex(row.str("gender")),
			"birth_date":               row.nullDate("birth_date"),
			"due_date":                 row.nullDate("due_date"),
			"gestational_age_weeks":    row.nullFloat64("gest_age"),
			"birth_weight":             row.nullFloat64("birth_weight"),
			"apgar_1":                  row.nullInt64("apgar_1"),
			"apgar_2":                  row.nullInt64("apgar_2"),
			"premie":                   triStatePremie(row.str("premie")),
			"birth_complications":      triStateBool(row.str("birth_comp")),
			"twin":                     triStateBool(row.str("twin")),
			"race_ethnicity":           mapRaceEthnicity(row.str("ethnicity"), row.str("race"), row.str("other_race")),
			"languages":                mapLanguages(row.str("languages")),
			"recruitment_source_id":    sourceID,
			"recruitment_source_other": row.str("source_other"),
			"response":                 response,
			"created_by_user_id":       creatorID,
			"inactive_reason":          row.str("inactive_reason"),
			"mcdi_percentile":          row.nullInt64("last_mcdi_pct"),
			"mcdi_date":                row.nullDate("last_mcdi_date"),
			"deactivated_at":           deactivatedAt(row.str("record_status"), importedAt),
		}

		err = withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			return insertRow(ctx, sp, "children", fields)
		})
		if err != nil {
			report.Error("children", id, err)
			continue
		}
		report.Ok("children")
	}
	return bumpSequence(ctx, tx, "children")
}
