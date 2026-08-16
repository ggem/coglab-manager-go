package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return hash
}

func TestPasswordAuthenticator_Authenticate_Success(t *testing.T) {
	hash := mustHash(t, "s3cret")
	q := &dbfake.Querier{
		GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
			return db.User{
				ID:           1,
				Email:        "researcher@example.edu",
				FirstName:    "Ada",
				LastName:     "Lovelace",
				PasswordHash: &hash,
			}, nil
		},
	}
	authr := NewPasswordAuthenticator(q)

	got, err := authr.Authenticate(context.Background(), "researcher@example.edu", "s3cret")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	want := Identity{UserID: 1, Email: "researcher@example.edu", FirstName: "Ada", LastName: "Lovelace"}
	if got != want {
		t.Errorf("Authenticate() = %+v, want %+v", got, want)
	}
}

func TestPasswordAuthenticator_Authenticate_WrongPassword(t *testing.T) {
	hash := mustHash(t, "s3cret")
	q := &dbfake.Querier{
		GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
			return db.User{Email: "researcher@example.edu", PasswordHash: &hash}, nil
		},
	}
	authr := NewPasswordAuthenticator(q)

	_, err := authr.Authenticate(context.Background(), "researcher@example.edu", "wrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Authenticate() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestPasswordAuthenticator_Authenticate_UnknownEmail(t *testing.T) {
	q := &dbfake.Querier{
		GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
	}
	authr := NewPasswordAuthenticator(q)

	_, err := authr.Authenticate(context.Background(), "nobody@example.edu", "whatever")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Authenticate() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestPasswordAuthenticator_Authenticate_Deactivated(t *testing.T) {
	hash := mustHash(t, "s3cret")
	q := &dbfake.Querier{
		GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
			return db.User{
				Email:         "former-staff@example.edu",
				PasswordHash:  &hash,
				DeactivatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}, nil
		},
	}
	authr := NewPasswordAuthenticator(q)

	_, err := authr.Authenticate(context.Background(), "former-staff@example.edu", "s3cret")
	if !errors.Is(err, ErrAccountDeactivated) {
		t.Errorf("Authenticate() error = %v, want ErrAccountDeactivated", err)
	}
}

func TestPasswordAuthenticator_Authenticate_SSOOnlyAccount(t *testing.T) {
	q := &dbfake.Querier{
		GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
			return db.User{Email: "sso-user@example.edu", PasswordHash: nil}, nil
		},
	}
	authr := NewPasswordAuthenticator(q)

	_, err := authr.Authenticate(context.Background(), "sso-user@example.edu", "anything")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Authenticate() error = %v, want ErrInvalidCredentials", err)
	}
}
