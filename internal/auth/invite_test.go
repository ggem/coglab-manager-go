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

func TestCreateInviteToken_StoresOnlyTheHash(t *testing.T) {
	var stored db.CreatePasswordSetTokenParams
	q := &dbfake.Querier{
		CreatePasswordSetTokenFunc: func(ctx context.Context, arg db.CreatePasswordSetTokenParams) (db.PasswordSetToken, error) {
			stored = arg
			return db.PasswordSetToken{ID: 1, TokenHash: arg.TokenHash, UserID: arg.UserID, ExpiresAt: arg.ExpiresAt}, nil
		},
	}

	token, err := CreateInviteToken(context.Background(), q, 7)
	if err != nil {
		t.Fatalf("CreateInviteToken: %v", err)
	}
	if token == "" {
		t.Fatal("CreateInviteToken returned an empty token")
	}
	if stored.UserID != 7 {
		t.Errorf("stored UserID = %d, want 7", stored.UserID)
	}
	if len(stored.TokenHash) == 0 {
		t.Error("stored TokenHash is empty")
	}
	if !stored.ExpiresAt.Valid || !stored.ExpiresAt.Time.After(time.Now()) {
		t.Errorf("stored ExpiresAt = %+v, want a valid future time", stored.ExpiresAt)
	}
}

func TestRedeemInviteToken_Success(t *testing.T) {
	var setPassword db.SetUserPasswordParams
	var claimedHash []byte
	q := &dbfake.Querier{
		ClaimPasswordSetTokenFunc: func(ctx context.Context, tokenHash []byte) (db.PasswordSetToken, error) {
			claimedHash = tokenHash
			return db.PasswordSetToken{
				ID: 42, UserID: 7,
				ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
				UsedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}, nil
		},
		GetUserByIDFunc: func(ctx context.Context, id int64) (db.User, error) {
			return db.User{ID: 7, Email: "new-hire@example.edu", FirstName: "Ada", LastName: "Lovelace"}, nil
		},
		SetUserPasswordFunc: func(ctx context.Context, arg db.SetUserPasswordParams) error {
			setPassword = arg
			return nil
		},
	}

	got, err := RedeemInviteToken(context.Background(), q, "some-raw-token", "a-new-password")
	if err != nil {
		t.Fatalf("RedeemInviteToken: %v", err)
	}
	want := Identity{UserID: 7, Email: "new-hire@example.edu", FirstName: "Ada", LastName: "Lovelace"}
	if got != want {
		t.Errorf("RedeemInviteToken() = %+v, want %+v", got, want)
	}
	if setPassword.ID != 7 || setPassword.PasswordHash == nil {
		t.Errorf("SetUserPassword params = %+v, want ID=7 with a non-nil hash", setPassword)
	}
	if len(claimedHash) == 0 {
		t.Error("ClaimPasswordSetToken called with an empty hash")
	}
}

// TestRedeemInviteToken_NotFound covers an unknown token -- indistinguishable
// (by design) from an expired or already-used one at the SQL level: the
// atomic claim query's WHERE clause simply matches no row in every case,
// same as Postgres would report for expired/already-used tokens (see the
// query's own doc comment) -- there's nothing left in Go to unit-test
// separately for those cases now that the check moved into the query.
func TestRedeemInviteToken_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		ClaimPasswordSetTokenFunc: func(ctx context.Context, tokenHash []byte) (db.PasswordSetToken, error) {
			return db.PasswordSetToken{}, pgx.ErrNoRows
		},
	}

	_, err := RedeemInviteToken(context.Background(), q, "unknown-token", "a-new-password")
	if !errors.Is(err, ErrInvalidInviteToken) {
		t.Errorf("RedeemInviteToken() error = %v, want ErrInvalidInviteToken", err)
	}
}

func TestRedeemInviteToken_DeactivatedAccount(t *testing.T) {
	q := &dbfake.Querier{
		ClaimPasswordSetTokenFunc: func(ctx context.Context, tokenHash []byte) (db.PasswordSetToken, error) {
			return db.PasswordSetToken{
				ID: 1, UserID: 7,
				ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
			}, nil
		},
		GetUserByIDFunc: func(ctx context.Context, id int64) (db.User, error) {
			return db.User{ID: 7, DeactivatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}, nil
		},
		SetUserPasswordFunc: func(ctx context.Context, arg db.SetUserPasswordParams) error {
			t.Fatal("should not set a password for a deactivated account")
			return nil
		},
	}

	_, err := RedeemInviteToken(context.Background(), q, "some-token", "a-new-password")
	if !errors.Is(err, ErrAccountDeactivated) {
		t.Errorf("RedeemInviteToken() error = %v, want ErrAccountDeactivated", err)
	}
}

// TestRedeemInviteToken_AlreadyActivated covers a stale token for an
// account that already has a password -- e.g. the loser of the race the
// atomic claim is meant to prevent (see ClaimPasswordSetToken's doc
// comment), or an earlier invite email surfacing after a resend. This
// flow is first-time activation only, not a general password-reset
// mechanism, so it must reject rather than silently overwrite the
// existing password.
func TestRedeemInviteToken_AlreadyActivated(t *testing.T) {
	existingHash := "existing-hash"
	q := &dbfake.Querier{
		ClaimPasswordSetTokenFunc: func(ctx context.Context, tokenHash []byte) (db.PasswordSetToken, error) {
			return db.PasswordSetToken{
				ID: 1, UserID: 7,
				ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
			}, nil
		},
		GetUserByIDFunc: func(ctx context.Context, id int64) (db.User, error) {
			return db.User{ID: 7, Email: "already-active@example.edu", PasswordHash: &existingHash}, nil
		},
		SetUserPasswordFunc: func(ctx context.Context, arg db.SetUserPasswordParams) error {
			t.Fatal("should not overwrite an existing password via a stale invite token")
			return nil
		},
	}

	_, err := RedeemInviteToken(context.Background(), q, "stale-token", "a-new-password")
	if !errors.Is(err, ErrInvalidInviteToken) {
		t.Errorf("RedeemInviteToken() error = %v, want ErrInvalidInviteToken", err)
	}
}
