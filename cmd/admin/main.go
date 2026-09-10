// cmd/admin bootstraps or promotes a platform admin directly against
// the database -- the one HTTP-free path into the account-creation
// feature, needed because the very first admin has no existing admin to
// grant them that role, and bootstrap time may predate SMTP even being
// configured, ruling out routing through the same email-invite flow the
// HTTP API otherwise always uses.
//
// Usage:
//
//	DATABASE_URL=... go run ./cmd/admin -email=admin@example.edu -first-name=Ada -last-name=Lovelace -password=<temp password>
//
// If a user with -email already exists, they're promoted to platform
// admin (and their password reset, since -password is required); if
// not, a new account is created directly with a real password_hash --
// the only caller in this codebase that ever sets a password outside
// the invite-redemption path, deliberately: this is an operator-run
// one-off, not part of the self-service flow every other account uses.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ggem/coglab-manager-go/internal/auth"
	"github.com/ggem/coglab-manager-go/internal/db"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	email := flag.String("email", "", "email of the account to create or promote (required)")
	firstName := flag.String("first-name", "", "first name, used only when creating a new account")
	lastName := flag.String("last-name", "", "last name, used only when creating a new account")
	password := flag.String("password", "", "sets (or resets) this account's local password (required)")
	flag.Parse()

	if *email == "" {
		return fmt.Errorf("-email is required")
	}
	if *password == "" {
		return fmt.Errorf("-password is required")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("create connection pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	queries := db.New(pool)

	hash, err := auth.HashPassword(*password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user, err := queries.GetUserByEmail(ctx, *email)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("look up user: %w", err)
		}
		if *firstName == "" || *lastName == "" {
			return fmt.Errorf("-first-name and -last-name are required when creating a new account")
		}
		user, err = queries.CreateUser(ctx, db.CreateUserParams{
			Email:           *email,
			FirstName:       *firstName,
			LastName:        *lastName,
			PasswordHash:    &hash,
			IsPlatformAdmin: true,
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		fmt.Printf("created platform admin %s (id %d)\n", user.Email, user.ID)
		return nil
	}

	if _, err := queries.SetUserPlatformAdmin(ctx, db.SetUserPlatformAdminParams{ID: user.ID, IsPlatformAdmin: true}); err != nil {
		return fmt.Errorf("promote user: %w", err)
	}
	if err := queries.SetUserPassword(ctx, db.SetUserPasswordParams{ID: user.ID, PasswordHash: &hash}); err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	fmt.Printf("promoted %s (id %d) to platform admin and reset their password\n", user.Email, user.ID)
	return nil
}
