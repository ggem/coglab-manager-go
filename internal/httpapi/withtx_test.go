package httpapi

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

// fakeTx is a minimal pgx.Tx double: it only implements what withTx and
// db.New's DBTX actually exercise, panicking on the rest -- the same
// "unset method panics" convention dbfake uses. This lets withTx's
// commit/rollback behavior be tested without a real database; actual
// atomicity against Postgres is exercised by the integration suite,
// which passes a real pool as the beginner.
type fakeTx struct {
	committed  bool
	rolledBack bool
}

func (t *fakeTx) Begin(ctx context.Context) (pgx.Tx, error) { panic("fakeTx: Begin not implemented") }

func (t *fakeTx) Commit(ctx context.Context) error {
	t.committed = true
	return nil
}

func (t *fakeTx) Rollback(ctx context.Context) error {
	if !t.committed {
		t.rolledBack = true
	}
	return pgx.ErrTxClosed
}

func (t *fakeTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	panic("fakeTx: CopyFrom not implemented")
}

func (t *fakeTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	panic("fakeTx: SendBatch not implemented")
}

func (t *fakeTx) LargeObjects() pgx.LargeObjects { panic("fakeTx: LargeObjects not implemented") }

func (t *fakeTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	panic("fakeTx: Prepare not implemented")
}

func (t *fakeTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	panic("fakeTx: Query not implemented")
}

func (t *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	panic("fakeTx: QueryRow not implemented")
}

func (t *fakeTx) Conn() *pgx.Conn { panic("fakeTx: Conn not implemented") }

// fakeBeginner hands out a single fakeTx so a test can inspect whether it
// was committed or rolled back afterward.
type fakeBeginner struct {
	tx *fakeTx
}

func (b *fakeBeginner) Begin(ctx context.Context) (pgx.Tx, error) {
	return b.tx, nil
}

func TestWithTx_CommitsOnSuccess(t *testing.T) {
	tx := &fakeTx{}
	s := &Server{beginner: &fakeBeginner{tx: tx}}

	err := s.withTx(context.Background(), func(q db.Querier) error {
		return nil
	})

	if err != nil {
		t.Fatalf("withTx: %v", err)
	}
	if !tx.committed {
		t.Error("transaction was not committed")
	}
	if tx.rolledBack {
		t.Error("transaction was rolled back despite fn succeeding")
	}
}

func TestWithTx_RollsBackOnError(t *testing.T) {
	tx := &fakeTx{}
	s := &Server{beginner: &fakeBeginner{tx: tx}}
	wantErr := assertErr("boom")

	err := s.withTx(context.Background(), func(q db.Querier) error {
		return wantErr
	})

	if err != error(wantErr) {
		t.Fatalf("withTx error = %v, want %v", err, wantErr)
	}
	if tx.committed {
		t.Error("transaction was committed despite fn returning an error")
	}
	if !tx.rolledBack {
		t.Error("transaction was not rolled back")
	}
}

func TestWithTx_NilBeginnerRunsDirectly(t *testing.T) {
	q := &dbfake.Querier{}
	s := &Server{queries: q}
	var gotQuerier db.Querier

	err := s.withTx(context.Background(), func(fnQ db.Querier) error {
		gotQuerier = fnQ
		return nil
	})

	if err != nil {
		t.Fatalf("withTx: %v", err)
	}
	if gotQuerier != db.Querier(q) {
		t.Error("withTx did not run fn against s.queries when beginner is nil")
	}
}
