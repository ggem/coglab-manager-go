package httpapi

import (
	"slices"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
)

// nihRaceCategoryLabels maps this app's race_ethnicity categories (see the
// check constraint on children.race_ethnicity) to the fixed vocabulary the
// current NIH participant-level data template allows in its Race column.
// hispanic_or_latino is excluded here -- it's an ethnicity flag, handled by
// nihEthnicityLabel, not a race category. middle_eastern_or_north_african
// has no category of its own in the template, so it's folded into White,
// matching the federal race categories the template's 7-value vocabulary
// still reflects.
var nihRaceCategoryLabels = map[string]string{
	"american_indian_or_alaska_native":    "American Indian",
	"asian":                               "Asian",
	"black_or_african_american":           "Black",
	"native_hawaiian_or_pacific_islander": "Hawaiian",
	"white":                               "White",
	"middle_eastern_or_north_african":     "White",
}

func nihSexLabel(sex string) string {
	switch sex {
	case "male":
		return "Male"
	case "female":
		return "Female"
	default:
		return "Unknown"
	}
}

func nihEthnicityLabel(categories []string) string {
	if len(categories) == 0 {
		return "Unknown"
	}
	if slices.Contains(categories, "hispanic_or_latino") {
		return "Hispanic or Latino"
	}
	return "Not Hispanic or Latino"
}

func nihRaceLabel(categories []string) string {
	if len(categories) == 0 {
		return "Unknown"
	}
	var labels []string
	for _, category := range categories {
		if label, ok := nihRaceCategoryLabels[category]; ok && !slices.Contains(labels, label) {
			labels = append(labels, label)
		}
	}
	switch len(labels) {
	case 0:
		// Only hispanic_or_latino (an ethnicity, not a race) was selected.
		return "Unknown"
	case 1:
		return labels[0]
	default:
		return "More than one race"
	}
}

// nihAge computes the Age/Age Unit pair for the NIH template: blank age
// with "Unknown" unit when birth_date isn't known, blank age with
// "Ninety Plus" for participants 90 years (1080 months) or older, and a
// whole-number month count otherwise -- the app already models age in
// months everywhere else (e.g. experiments.age_range_min/max_months), so
// that's the unit used here too rather than switching to years.
func nihAge(birthDate, scheduleDate pgtype.Date) (age, ageUnit string) {
	if !birthDate.Valid {
		return "", "Unknown"
	}
	years := scheduleDate.Time.Year() - birthDate.Time.Year()
	months := int(scheduleDate.Time.Month()) - int(birthDate.Time.Month())
	totalMonths := years*12 + months
	if scheduleDate.Time.Day() < birthDate.Time.Day() {
		totalMonths--
	}
	if totalMonths < 0 {
		totalMonths = 0
	}
	if totalMonths >= 1080 {
		return "", "Ninety Plus"
	}
	return strconv.Itoa(totalMonths), "Months"
}
