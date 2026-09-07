package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// insertRow inserts one row into an imported table with its legacy
// primary key preserved verbatim as fields["id"] -- OVERRIDING SYSTEM
// VALUE lets an explicit value go into a "generated always as identity"
// column without altering the column's identity mode (which would weaken
// the same protection against accidental id collisions every other,
// non-imported insert path in this app relies on). fields is a
// map[string]any rather than a positional column/value pair so call
// sites read as a plain struct literal; keys are sorted for a
// deterministic, diffable query across runs.
func insertRow(ctx context.Context, tx pgx.Tx, table string, fields map[string]any) error {
	cols := make([]string, 0, len(fields))
	for c := range fields {
		cols = append(cols, c)
	}
	sort.Strings(cols)

	placeholders := make([]string, len(cols))
	values := make([]any, len(cols))
	for i, c := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		values[i] = fields[c]
	}

	query := fmt.Sprintf("insert into %s (%s) overriding system value values (%s)",
		table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	if _, err := tx.Exec(ctx, query, values...); err != nil {
		return fmt.Errorf("insert into %s: %w", table, err)
	}
	return nil
}

// insertAssoc inserts one row into a plain association/join table (no
// "id" column of its own, so no identity override needed).
func insertAssoc(ctx context.Context, tx pgx.Tx, table string, fields map[string]any) error {
	cols := make([]string, 0, len(fields))
	for c := range fields {
		cols = append(cols, c)
	}
	sort.Strings(cols)

	placeholders := make([]string, len(cols))
	values := make([]any, len(cols))
	for i, c := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		values[i] = fields[c]
	}

	query := fmt.Sprintf("insert into %s (%s) values (%s)",
		table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	if _, err := tx.Exec(ctx, query, values...); err != nil {
		return fmt.Errorf("insert into %s: %w", table, err)
	}
	return nil
}

// withSavepoint runs fn inside a savepoint nested within tx (pgx
// implements a nested Begin on an existing Tx as SAVEPOINT/RELEASE
// SAVEPOINT under the hood). This is why every row-level import goes
// through here rather than inserting directly against the table's
// shared per-table transaction: once any statement in a Postgres
// transaction fails, the whole transaction is aborted and every later
// statement fails too ("current transaction is aborted") until a
// rollback -- without a savepoint per row, the first bad row (a bad
// date, an orphaned FK -- expected in 20-year-old hand-entered data)
// would cascade spurious failures onto every row after it instead of
// being cleanly skipped and reported on its own.
func withSavepoint(ctx context.Context, tx pgx.Tx, fn func(pgx.Tx) error) error {
	sp, err := tx.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin savepoint: %w", err)
	}
	if err := fn(sp); err != nil {
		if rbErr := sp.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("%w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}
	return sp.Commit(ctx)
}

// bumpSequence resets table's identity sequence to start after the
// highest imported id, so the app's normal (non-import) insert paths --
// which never specify an id -- don't collide with an imported row.
// Called once per table after all its rows are inserted.
func bumpSequence(ctx context.Context, tx pgx.Tx, table string) error {
	query := fmt.Sprintf(
		"select setval(pg_get_serial_sequence('%s', 'id'), coalesce((select max(id) from %s), 1))",
		table, table)
	if _, err := tx.Exec(ctx, query); err != nil {
		return fmt.Errorf("bump sequence for %s: %w", table, err)
	}
	return nil
}
