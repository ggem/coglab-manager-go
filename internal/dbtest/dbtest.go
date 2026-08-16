// Package dbtest starts a disposable Postgres container and applies the
// project's migrations to it, so integration tests can run against a real
// database without a manually-managed fixture. It requires Docker (or
// Colima) to be running.
package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Postgres is a running, migrated Postgres instance for integration tests.
type Postgres struct {
	// Pool is a ready-to-use connection pool, suitable for passing to
	// db.New.
	Pool *pgxpool.Pool

	container *postgres.PostgresContainer
	migrateDB *sql.DB
}

// StartPostgres starts a Postgres container and applies every migration in
// the project's migrations directory. Call Close (typically via
// t.Cleanup) to tear it down.
func StartPostgres(ctx context.Context) (*Postgres, error) {
	container, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("coglab_test"),
		postgres.WithUsername("coglab"),
		postgres.WithPassword("coglab"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("get connection string: %w", err)
	}

	// goose operates on a database/sql.DB; the pool below is separate and
	// is what tests actually query through, matching how cmd/api uses
	// pgxpool.
	migrateDB, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, fmt.Errorf("open migration connection: %w", err)
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return nil, fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.RunContext(ctx, "up", migrateDB, migrationsDir()); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	return &Postgres{Pool: pool, container: container, migrateDB: migrateDB}, nil
}

// Close tears down the connection pool, migration connection, and
// container, in that order.
func (p *Postgres) Close(ctx context.Context) error {
	p.Pool.Close()
	if err := p.migrateDB.Close(); err != nil {
		return fmt.Errorf("close migration connection: %w", err)
	}
	return p.container.Terminate(ctx)
}

// migrationsDir resolves to the repository's migrations directory
// regardless of which package this is called from, since `go test` sets
// the working directory to the package under test.
func migrationsDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")
}
