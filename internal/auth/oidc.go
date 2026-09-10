package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"

	"github.com/ggem/coglab-manager-go/internal/db"
)

// OIDCConfig is the four values a specific institution's IdP registration
// gives you -- everything else (endpoints, keys) OIDCAuthenticator
// discovers itself from IssuerURL at construction time.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// SSOAuthenticator is the interface httpapi's SSO handlers depend on,
// same rationale as LocalAuthenticator: a caller might plausibly want to
// substitute a test double for the real *OIDCAuthenticator (which needs
// live OIDC discovery and a real ID token to exercise for real).
type SSOAuthenticator interface {
	AuthCodeURL(state, nonce string) string
	Exchange(ctx context.Context, code, nonce string) (Identity, error)
}

// OIDCAuthenticator is the SSO counterpart to PasswordAuthenticator: a
// structurally different exchange (redirect-and-callback, not a single
// Authenticate(email, password) call), but it resolves to the same
// Identity type, which is what the rest of this package and httpapi's
// login handling actually care about.
type OIDCAuthenticator struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth2   oauth2.Config
	queries  db.Querier
}

// NewOIDCAuthenticator runs OIDC discovery against cfg.IssuerURL once, up
// front -- an unreachable or misconfigured issuer fails server startup
// immediately, the same fail-fast-on-bad-config convention this codebase
// already applies to MCDI/SMTP setup, rather than surfacing as a mystery
// 500 on someone's first SSO login attempt.
func NewOIDCAuthenticator(ctx context.Context, cfg OIDCConfig, queries db.Querier) (*OIDCAuthenticator, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery for %q: %w", cfg.IssuerURL, err)
	}

	return &OIDCAuthenticator{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth2: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
		queries: queries,
	}, nil
}

// AuthCodeURL is where handleSSOLogin redirects the browser to start the
// flow. nonce is echoed back inside the ID token itself (not just the
// callback URL, unlike state) -- Exchange checks it to prove the token
// being verified was actually issued for this specific login attempt.
func (a *OIDCAuthenticator) AuthCodeURL(state, nonce string) string {
	return a.oauth2.AuthCodeURL(state, oidc.Nonce(nonce))
}

// ssoClaims is deliberately a subset of the standard OIDC claims: this app
// only needs enough to identify the person and pre-fill a new account.
type ssoClaims struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Name          string `json:"name"`
}

// validate checks the claims this app actually depends on for identity
// resolution. email_verified is required, not merely inspected -- without
// it, an IdP that asserts an unconfirmed address (or simply omits the
// claim) could take over an existing local-password account through the
// email-match fallback in resolveUser, by claiming someone else's email.
func (c ssoClaims) validate() error {
	if c.Subject == "" {
		return errors.New("id_token has no sub claim")
	}
	if c.Email == "" {
		return errors.New("id_token has no email claim")
	}
	if !c.EmailVerified {
		return errors.New("id_token email is not verified")
	}
	return nil
}

func (c ssoClaims) names() (first, last string) {
	if c.GivenName != "" || c.FamilyName != "" {
		return c.GivenName, c.FamilyName
	}
	// Fall back to splitting the display name -- not every IdP sends
	// given_name/family_name separately. Good enough for a first-login
	// default; the person can correct it afterward like any other user.
	if parts := strings.Fields(c.Name); len(parts) > 0 {
		if len(parts) == 1 {
			return parts[0], ""
		}
		return parts[0], strings.Join(parts[1:], " ")
	}
	return "", ""
}

// Exchange completes the callback half of the flow: trades code for
// tokens, verifies the ID token's signature and nonce, and resolves the
// claims to a users row -- by sso_subject first, then by email (backfilling
// sso_subject onto that existing local-password account so it doesn't
// silently create a duplicate), and only creates a brand-new row if
// neither matches.
func (a *OIDCAuthenticator) Exchange(ctx context.Context, code, expectedNonce string) (Identity, error) {
	token, err := a.oauth2.Exchange(ctx, code)
	if err != nil {
		return Identity{}, fmt.Errorf("exchange code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return Identity{}, errors.New("token response has no id_token")
	}
	idToken, err := a.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Identity{}, fmt.Errorf("verify id_token: %w", err)
	}
	if idToken.Nonce != expectedNonce {
		return Identity{}, errors.New("id_token nonce does not match")
	}

	var claims ssoClaims
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("parse id_token claims: %w", err)
	}
	if err := claims.validate(); err != nil {
		return Identity{}, err
	}

	return a.resolveUser(ctx, idToken.Issuer, claims)
}

// resolveUser looks up (or links, or creates) the users row for this
// identity. issuer + claims.Subject together are the identity key, not
// claims.Subject alone -- sub is only guaranteed unique *within* one
// issuer, and an institution that migrates to a different IdP over the
// years could otherwise, in principle, end up with two different
// providers issuing the same sub to two different people.
func (a *OIDCAuthenticator) resolveUser(ctx context.Context, issuer string, claims ssoClaims) (Identity, error) {
	sub := claims.Subject

	if user, err := a.queries.GetUserBySSOIdentity(ctx, db.GetUserBySSOIdentityParams{SsoIssuer: &issuer, SsoSubject: &sub}); err == nil {
		return identityFromUser(user)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, fmt.Errorf("get user by sso identity: %w", err)
	}

	if user, err := a.queries.GetUserByEmail(ctx, claims.Email); err == nil {
		if user.DeactivatedAt.Valid {
			// Don't link (or reveal via a different error) a deactivated
			// account's identity -- same "just reject" outcome
			// identityFromUser would give a live account.
			return Identity{}, ErrAccountDeactivated
		}
		if err := a.queries.SetUserSSOIdentity(ctx, db.SetUserSSOIdentityParams{ID: user.ID, SsoIssuer: &issuer, SsoSubject: &sub}); err != nil {
			return Identity{}, fmt.Errorf("link sso identity to existing account: %w", err)
		}
		return identityFromUser(user)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, fmt.Errorf("get user by email: %w", err)
	}

	first, last := claims.names()
	user, err := a.queries.CreateUser(ctx, db.CreateUserParams{
		Email:           claims.Email,
		FirstName:       first,
		LastName:        last,
		PasswordHash:    nil,
		IsPlatformAdmin: false,
		SsoIssuer:       &issuer,
		SsoSubject:      &sub,
	})
	if err != nil {
		return Identity{}, fmt.Errorf("create user from sso claims: %w", err)
	}
	return identityFromUser(user)
}

func identityFromUser(user db.User) (Identity, error) {
	if user.DeactivatedAt.Valid {
		return Identity{}, ErrAccountDeactivated
	}
	return Identity{
		UserID:          user.ID,
		Email:           user.Email,
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		IsPlatformAdmin: user.IsPlatformAdmin,
	}, nil
}
