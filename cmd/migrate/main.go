package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("dir", "migrations", "directory containing migration files")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		return fmt.Errorf("usage: migrate [-dir=migrations] <command> [args...]\ncommands: up, up-by-one, down, redo, status, version, create <name> sql")
	}
	command, commandArgs := args[0], args[1:]

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	// "create" only scaffolds a local file and never touches the database,
	// so it shouldn't require DATABASE_URL to be set.
	if command == "create" {
		return goose.RunContext(context.Background(), command, nil, *dir, commandArgs...)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable is required")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	return goose.RunContext(context.Background(), command, db, *dir, commandArgs...)
}
