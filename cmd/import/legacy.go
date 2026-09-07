package main

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"
)

// legacyRow is one row from the legacy kids_subjects database, keyed by
// column name rather than a per-table typed struct: with ~30 tables and
// dozens of columns each, hand-written positional Scan calls would be
// both huge and fragile against column reordering. Each domain file's
// mapping function pulls out only the columns it needs via the typed
// accessors below, which double as the single place legacy's MySQL
// NULL/enum/date conventions get normalized into Go values.
type legacyRow map[string]any

// queryLegacy runs query against the legacy database and returns every
// row as a legacyRow, converting driver []byte values (MySQL's driver
// returns these for most text/enum/char columns) to string so accessor
// methods never have to type-switch on the driver's underlying wire
// representation.
func queryLegacy(ctx context.Context, legacyDB *sql.DB, query string, args ...any) ([]legacyRow, error) {
	rows, err := legacyDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query legacy %q: %w", query, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}

	var result []legacyRow
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		row := make(legacyRow, len(cols))
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				row[c] = string(b)
			} else {
				row[c] = vals[i]
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// str returns a non-nullable text/enum/char column as-is ("" if the
// column is unexpectedly NULL -- every such legacy column in this schema
// is declared "not null default ...", so real NULLs shouldn't occur).
func (r legacyRow) str(col string) string {
	v := r[col]
	if v == nil {
		return ""
	}
	return v.(string)
}

// nullStr returns a genuinely nullable text/enum column (declared
// without "not null" in the legacy schema), nil when NULL.
func (r legacyRow) nullStr(col string) *string {
	v := r[col]
	if v == nil {
		return nil
	}
	s := v.(string)
	return &s
}

// int64 returns a non-nullable integer column.
func (r legacyRow) int64(col string) int64 {
	v := r[col]
	if v == nil {
		return 0
	}
	return v.(int64)
}

// nullInt64 returns a nullable integer column, nil when NULL.
func (r legacyRow) nullInt64(col string) *int64 {
	v := r[col]
	if v == nil {
		return nil
	}
	n := v.(int64)
	return &n
}

// nullFloat64 returns a nullable double(8,5)-style column, nil when
// NULL -- every legacy numeric column this importer reads (gest_age,
// birth_weight, age_range_min/max, ...) is nullable.
func (r legacyRow) nullFloat64(col string) *float64 {
	v := r[col]
	if v == nil {
		return nil
	}
	switch n := v.(type) {
	case float64:
		return &n
	case string:
		// Some MySQL configurations return DECIMAL columns as strings;
		// legacy's double(8,5) columns are safe to reparse as float64.
		var f float64
		if _, err := fmt.Sscanf(n, "%g", &f); err != nil {
			return nil
		}
		return &f
	default:
		return nil
	}
}

// nullDate returns a nullable date column as a time.Time, nil when NULL
// -- the legacy driver DSN must include parseTime=true for this to work.
func (r legacyRow) nullDate(col string) *time.Time {
	v := r[col]
	if v == nil {
		return nil
	}
	t := v.(time.Time)
	return &t
}

var enumLiteral = regexp.MustCompile(`'((?:[^'\\]|\\.)*)'`)

// legacyEnumValues reads an enum column's actual, currently-live
// definition straight from MySQL's information_schema rather than
// trusting any hardcoded snapshot of it -- this database's schema has
// genuinely drifted from the create-database reference script over its
// ~20-year life (confirmed: children.source has grown from a
// documented 26 values to 61 live ones, appointments.schedule_status
// has gained an undocumented "rescheduled", parents.education_1 has
// both a renamed value and a wholly new "Graduate Degree"). Returns the
// enum's values in their declared order.
func legacyEnumValues(ctx context.Context, legacyDB *sql.DB, table, column string) ([]string, error) {
	var columnType string
	err := legacyDB.QueryRowContext(ctx,
		`select column_type from information_schema.columns
		 where table_schema = database() and table_name = ? and column_name = ?`,
		table, column,
	).Scan(&columnType)
	if err != nil {
		return nil, fmt.Errorf("read enum definition for %s.%s: %w", table, column, err)
	}
	matches := enumLiteral.FindAllStringSubmatch(columnType, -1)
	values := make([]string, len(matches))
	for i, m := range matches {
		values[i] = m[1]
	}
	return values, nil
}

// validateEnumCoverage fails fast (before any row is imported) if the
// live enum contains a value mapped isn't prepared for, rather than
// discovering it thousands of rows into a run as a wall of identical
// per-row errors. mapped only needs to answer "do I have a mapping for
// this legacy value", not provide it -- callers pass a plain lookup
// function so this works for both map[string]string-backed and
// switch-statement-backed mappers.
func validateEnumCoverage(ctx context.Context, legacyDB *sql.DB, table, column string, mapped func(string) bool) error {
	values, err := legacyEnumValues(ctx, legacyDB, table, column)
	if err != nil {
		return err
	}
	var unmapped []string
	for _, v := range values {
		if !mapped(v) {
			unmapped = append(unmapped, v)
		}
	}
	if len(unmapped) > 0 {
		return fmt.Errorf("%s.%s has %d value(s) with no mapping: %q -- add them before importing", table, column, len(unmapped), unmapped)
	}
	return nil
}

// nullTimeStr returns a nullable MySQL TIME column ("15:04:05") as-is,
// nil when NULL -- unlike DATE/DATETIME, parseTime doesn't affect TIME
// columns, so the driver already returns these as text (converted to
// string by queryLegacy's []byte handling), which Postgres's time input
// parser accepts directly as a query parameter.
func (r legacyRow) nullTimeStr(col string) *string {
	return r.nullStr(col)
}
