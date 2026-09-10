package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
)

// inviteTokenTTL is generous (a week) compared to the OIDC state cookie's
// 10 minutes -- this is an email a person might not open right away, not
// a live redirect round trip.
const inviteTokenTTL = 7 * 24 * time.Hour

// ErrInvalidInviteToken covers every way a redemption can fail --
// unknown, expired, or already-used -- collapsed into one error so a
// caller can't distinguish them (same "don't reveal" convention as
// ErrInvalidCredentials on the login path).
var ErrInvalidInviteToken = errors.New("invalid or expired invite token")

// CreateInviteToken generates a password-set token for userID (a
// newly-created account with no password_hash yet) and stores its hash,
// reusing generateToken/hashToken from session.go -- same shape as a
// session token, but persisted in password_set_tokens with an expiry and
// a single-use marker instead of sessions' revocable-until-logout model.
// The raw token is returned so the caller can build the emailed link;
// only its hash is ever persisted.
func CreateInviteToken(ctx context.Context, queries db.Querier, userID int64) (string, error) {
	token, tokenHash, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	if _, err := queries.CreatePasswordSetToken(ctx, db.CreatePasswordSetTokenParams{
		TokenHash: tokenHash,
		UserID:    userID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(inviteTokenTTL), Valid: true},
	}); err != nil {
		return "", fmt.Errorf("create password set token: %w", err)
	}

	return token, nil
}

// RedeemInviteToken verifies token, sets newPassword on the account it
// names, and marks the token used so it can't be replayed. Not-found,
// expired, and already-used all collapse to the same ErrInvalidInviteToken
// -- same "don't reveal which case" convention as the login-failure path.
//
// The claim (ClaimPasswordSetToken) is a single atomic UPDATE ... WHERE
// used_at is null ... RETURNING, not a SELECT followed by a later
// UPDATE: two concurrent redemptions of the same token both racing
// against a plain SELECT could both observe "not yet used" and both
// proceed, each setting the password to a different value with neither
// erroring. The atomic claim closes that window -- only one caller's
// UPDATE can ever match the still-unclaimed row.
//
// Callers should run this inside a transaction (see httpapi's use of
// withTx): if HashPassword or SetUserPassword fails after the token is
// already claimed, rolling back the whole transaction un-claims it too,
// so the link remains redeemable on retry rather than being silently
// burned with no password ever set.
func RedeemInviteToken(ctx context.Context, queries db.Querier, token, newPassword string) (Identity, error) {
	record, err := queries.ClaimPasswordSetToken(ctx, hashToken(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, ErrInvalidInviteToken
		}
		return Identity{}, fmt.Errorf("claim password set token: %w", err)
	}

	user, err := queries.GetUserByID(ctx, record.UserID)
	if err != nil {
		return Identity{}, fmt.Errorf("get user: %w", err)
	}
	if user.DeactivatedAt.Valid {
		return Identity{}, ErrAccountDeactivated
	}
	if user.PasswordHash != nil {
		// The account already has a password -- this token is a stale
		// leftover (e.g. an earlier invite email found after a resend, or
		// the loser of the race the atomic claim above now prevents).
		// This flow is for first-time activation only, not a general
		// password-reset mechanism, so treat it as any other invalid
		// token rather than silently overwriting an existing password.
		return Identity{}, ErrInvalidInviteToken
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return Identity{}, fmt.Errorf("hash password: %w", err)
	}
	if err := queries.SetUserPassword(ctx, db.SetUserPasswordParams{ID: user.ID, PasswordHash: &hash}); err != nil {
		return Identity{}, fmt.Errorf("set password: %w", err)
	}

	return Identity{
		UserID:          user.ID,
		Email:           user.Email,
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		IsPlatformAdmin: user.IsPlatformAdmin,
	}, nil
}
