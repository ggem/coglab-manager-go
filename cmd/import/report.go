package main

import (
	"fmt"
	"io"
	"sort"
)

// rowError is one row's validation/transform failure, kept alongside
// its legacy primary key so a person reviewing the dry-run report can
// go find the actual bad record in the legacy database.
type rowError struct {
	table    string
	legacyID int64
	err      error
}

// Report collects counts and per-row errors across every imported
// table, so one bad row never aborts the whole run -- the point of the
// dry-run/report mode: surface everything wrong with the data in one
// pass rather than fail-fix-rerun one row at a time.
type Report struct {
	tables []string
	seen   map[string]bool
	ok     map[string]int
	errs   []rowError
}

func NewReport() *Report {
	return &Report{seen: map[string]bool{}, ok: map[string]int{}}
}

func (r *Report) touch(table string) {
	if !r.seen[table] {
		r.seen[table] = true
		r.tables = append(r.tables, table)
	}
}

// Ok records one successfully validated/imported row for table.
func (r *Report) Ok(table string) {
	r.touch(table)
	r.ok[table]++
}

// Error records one row that failed validation/transform and was
// skipped -- table import continues with the next row.
func (r *Report) Error(table string, legacyID int64, err error) {
	r.touch(table)
	r.errs = append(r.errs, rowError{table: table, legacyID: legacyID, err: err})
}

// HasErrors reports whether any row across any table failed -- the
// importer refuses to run for real (-dry-run=false) when this is true,
// so a partially-bad table can't silently import short.
func (r *Report) HasErrors() bool {
	return len(r.errs) > 0
}

// Write prints a per-table summary (rows ok / rows failed) followed by
// every individual error, sorted by table then legacy id so repeated
// runs against the same data produce a stable, diffable report.
func (r *Report) Write(w io.Writer) {
	fmt.Fprintln(w, "table               ok      failed")
	fmt.Fprintln(w, "-----               --      ------")
	for _, table := range r.tables {
		failed := 0
		for _, e := range r.errs {
			if e.table == table {
				failed++
			}
		}
		fmt.Fprintf(w, "%-20s%-8d%d\n", table, r.ok[table], failed)
	}

	if len(r.errs) == 0 {
		return
	}
	fmt.Fprintln(w, "\nerrors:")
	sorted := make([]rowError, len(r.errs))
	copy(sorted, r.errs)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].table != sorted[j].table {
			return sorted[i].table < sorted[j].table
		}
		return sorted[i].legacyID < sorted[j].legacyID
	})
	for _, e := range sorted {
		fmt.Fprintf(w, "  %s #%d: %v\n", e.table, e.legacyID, e.err)
	}
}
