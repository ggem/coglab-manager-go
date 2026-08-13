// Package auth handles verifying who a request is from. LocalAuthenticator
// (this file) covers email/password login; a future SSO milestone will add
// a separate interface for the OIDC/SAML redirect-and-callback flow, since
// that's a structurally different exchange, not just another backend for
// the same "Authenticate(credentials)" call. Both funnel into the same
// Identity type, which is the real seam between "how we verified who this
// is" and "the User row in our database."
package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/ggem/coglab-manager-go/internal/db"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDeactivated = errors.New("account is deactivated")
)

// Identity is what an authentication method resolves a set of credentials
// to, independent of how that resolution happened.
type Identity struct {
	Email     string
	FirstName string
	LastName  string
}

// LocalAuthenticator verifies an email/password pair against stored
// credentials.
type LocalAuthenticator interface {
	Authenticate(ctx context.Context, email, password string) (Identity, error)
}

// dummyHash is a valid Argon2id hash with no corresponding real password.
// Authenticate verifies against it whenever there's no real hash to check
// (unknown email, or an SSO-only account with no local password), so that
// those cases take about as long to reject as a wrong password for a real
// account. Without this, the extra time a real Argon2id verification takes
// compared to an early return would let an attacker distinguish valid
// emails from invalid ones purely from response timing.
var dummyHash string

func init() {
	h, err := HashPassword("")
	if err != nil {
		panic("auth: failed to precompute dummy hash: " + err.Error())
	}
	dummyHash = h
}

// PasswordAuthenticator is the local (non-SSO) LocalAuthenticator
// implementation, backed by the users table.
type PasswordAuthenticator struct {
	queries db.Querier
}

func NewPasswordAuthenticator(queries db.Querier) *PasswordAuthenticator {
	return &PasswordAuthenticator{queries: queries}
}

func (a *PasswordAuthenticator) Authenticate(ctx context.Context, email, password string) (Identity, error) {
	user, err := a.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_ = VerifyPassword(dummyHash, password)
			return Identity{}, ErrInvalidCredentials
		}
		return Identity{}, fmt.Errorf("get user by email: %w", err)
	}

	if user.DeactivatedAt.Valid {
		return Identity{}, ErrAccountDeactivated
	}

	if user.PasswordHash == nil {
		_ = VerifyPassword(dummyHash, password)
		return Identity{}, ErrInvalidCredentials
	}

	if err := VerifyPassword(*user.PasswordHash, password); err != nil {
		if errors.Is(err, ErrPasswordMismatch) {
			return Identity{}, ErrInvalidCredentials
		}
		return Identity{}, fmt.Errorf("verify password: %w", err)
	}

	return Identity{
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}, nil
}
