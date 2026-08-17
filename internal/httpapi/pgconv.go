package httpapi

import (
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
