package httpapi

import (
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// This file converts between pgx's pgtype wrappers and plain Go types for
// JSON request/response bodies. Kept separate from the db package's
// generated types deliberately, even though pgtype.Date/pgtype.Numeric
// already implement json.Marshaler/Unmarshaler: the wire format of a
// public API shouldn't be coupled to a specific SQL driver library's
// internal representation.

const dateLayout = "2006-01-02"

func dateToPtr(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format(dateLayout)
	return &s
}

func ptrToDate(s *string) (pgtype.Date, error) {
	if s == nil {
		return pgtype.Date{}, nil
	}
	t, err := time.Parse(dateLayout, *s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func numericToPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	return &f.Float64
}

func ptrToNumeric(f *float64) (pgtype.Numeric, error) {
	if f == nil {
		return pgtype.Numeric{}, nil
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(*f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

// clockLayout is a plain "HH:MM" time of day -- used for
// lab_availability_general/specific and schedule_blockings, which store a
// not-null time (unlike birth_date etc., there's no meaningful "unset"
// state for these, so no pointer/nullable variant is needed).
const clockLayout = "15:04"

func clockTimeToString(t pgtype.Time) string {
	return durationToClock(pgTimeToDuration(t))
}

// durationToClock formats a duration-since-midnight as "HH:MM" -- used
// both for pgtype.Time wire formatting and for internal/scheduling's
// plain time.Duration results (e.g. CandidateSlot.StartTime), which have
// no pgtype involved at all.
func durationToClock(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d", d/time.Hour, (d%time.Hour)/time.Minute)
}

func stringToClockTime(s string) (pgtype.Time, error) {
	t, err := time.Parse(clockLayout, s)
	if err != nil {
		return pgtype.Time{}, err
	}
	d := time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute
	return pgtype.Time{Microseconds: int64(d / time.Microsecond), Valid: true}, nil
}

// pgTimeToDuration converts a pgtype.Time (already a duration-since-
// midnight internally) to a plain time.Duration -- the representation
// internal/scheduling.TimeRange uses, keeping that package free of pgx
// awareness.
func pgTimeToDuration(t pgtype.Time) time.Duration {
	return time.Duration(t.Microseconds) * time.Microsecond
}
