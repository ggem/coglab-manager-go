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

// resolveUser only touches a.queries -- provider/verifier/oauth2 are
// nil-safe to leave unset in these tests, since Exchange (which does use
// them, and needs a real ID token to verify) isn't under test here.

func TestOIDCAuthenticator_ResolveUser_MatchByIssuerAndSubject(t *testing.T) {
	q := &dbfake.Querier{
		GetUserBySSOIdentityFunc: func(ctx context.Context, arg db.GetUserBySSOIdentityParams) (db.User, error) {
			if arg.SsoIssuer == nil || *arg.SsoIssuer != "https://idp.example.edu" || arg.SsoSubject == nil || *arg.SsoSubject != "sub-123" {
				t.Fatalf("GetUserBySSOIdentity called with %+v, want issuer=https://idp.example.edu subject=sub-123", arg)
			}
			return db.User{ID: 1, Email: "ada@example.edu", FirstName: "Ada", LastName: "Lovelace"}, nil
		},
		GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
			t.Fatal("should not fall back to email lookup when sso identity already matches")
			return db.User{}, nil
		},
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			t.Fatal("should not create a new user when sso identity already matches")
			return db.User{}, nil
		},
	}
	authr := &OIDCAuthenticator{queries: q}

	got, err := authr.resolveUser(context.Background(), "https://idp.example.edu", ssoClaims{Subject: "sub-123", Email: "ada@example.edu", EmailVerified: true})
	if err != nil {
		t.Fatalf("resolveUser: %v", err)
	}
	want := Identity{UserID: 1, Email: "ada@example.edu", FirstName: "Ada", LastName: "Lovelace"}
	if got != want {
		t.Errorf("resolveUser() = %+v, want %+v", got, want)
	}
}

func TestOIDCAuthenticator_ResolveUser_SameSubjectDifferentIssuer_DoesNotMatch(t *testing.T) {
	// A different issuer asserting the same sub value as some other,
	// unrelated provider must not resolve to that provider's user -- this
	// is exactly the collision the (issuer, sub) composite key exists to
	// rule out.
	q := &dbfake.Querier{
		GetUserBySSOIdentityFunc: func(ctx context.Context, arg db.GetUserBySSOIdentityParams) (db.User, error) {
			if arg.SsoIssuer == nil || *arg.SsoIssuer != "https://new-idp.example.edu" {
				t.Fatalf("GetUserBySSOIdentity issuer = %v, want https://new-idp.example.edu", arg.SsoIssuer)
			}
			return db.User{}, pgx.ErrNoRows
		},
		GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			if arg.SsoIssuer == nil || *arg.SsoIssuer != "https://new-idp.example.edu" {
				t.Errorf("CreateUser SsoIssuer = %v, want https://new-idp.example.edu", arg.SsoIssuer)
			}
			return db.User{ID: 55, Email: arg.Email}, nil
		},
	}
	authr := &OIDCAuthenticator{queries: q}

	// Same sub value ("sub-123") a different issuer previously used in
	// another test, but under a new issuer -- must be treated as an
	// unrelated identity, not matched to the old provider's user.
	_, err := authr.resolveUser(context.Background(), "https://new-idp.example.edu", ssoClaims{Subject: "sub-123", Email: "new-person@example.edu", EmailVerified: true})
	if err != nil {
		t.Fatalf("resolveUser: %v", err)
	}
}

func TestOIDCAuthenticator_ResolveUser_MatchByEmail_BackfillsIdentity(t *testing.T) {
	var linked db.SetUserSSOIdentityParams
	q := &dbfake.Querier{
		GetUserBySSOIdentityFunc: func(ctx context.Context, arg db.GetUserBySSOIdentityParams) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
		GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
			if email != "existing@example.edu" {
				t.Fatalf("GetUserByEmail called with %q", email)
			}
			return db.User{ID: 7, Email: "existing@example.edu", FirstName: "Grace", LastName: "Hopper"}, nil
		},
		SetUserSSOIdentityFunc: func(ctx context.Context, arg db.SetUserSSOIdentityParams) error {
			linked = arg
			return nil
		},
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			t.Fatal("should not create a new user when an existing account matches by email")
			return db.User{}, nil
		},
	}
	authr := &OIDCAuthenticator{queries: q}

	got, err := authr.resolveUser(context.Background(), "https://idp.example.edu", ssoClaims{Subject: "sub-456", Email: "existing@example.edu", EmailVerified: true})
	if err != nil {
		t.Fatalf("resolveUser: %v", err)
	}
	want := Identity{UserID: 7, Email: "existing@example.edu", FirstName: "Grace", LastName: "Hopper"}
	if got != want {
		t.Errorf("resolveUser() = %+v, want %+v", got, want)
	}
	if linked.ID != 7 || linked.SsoIssuer == nil || *linked.SsoIssuer != "https://idp.example.edu" || linked.SsoSubject == nil || *linked.SsoSubject != "sub-456" {
		t.Errorf("SetUserSSOIdentity params = %+v, want ID=7 SsoIssuer=https://idp.example.edu SsoSubject=sub-456", linked)
	}
}

func TestOIDCAuthenticator_ResolveUser_CreatesNewUser(t *testing.T) {
	var created db.CreateUserParams
	q := &dbfake.Querier{
		GetUserBySSOIdentityFunc: func(ctx context.Context, arg db.GetUserBySSOIdentityParams) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
		GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			created = arg
			return db.User{ID: 99, Email: arg.Email, FirstName: arg.FirstName, LastName: arg.LastName}, nil
		},
	}
	authr := &OIDCAuthenticator{queries: q}

	got, err := authr.resolveUser(context.Background(), "https://idp.example.edu", ssoClaims{
		Subject: "sub-789", Email: "new-hire@example.edu", EmailVerified: true, GivenName: "Barbara", FamilyName: "Liskov",
	})
	if err != nil {
		t.Fatalf("resolveUser: %v", err)
	}
	want := Identity{UserID: 99, Email: "new-hire@example.edu", FirstName: "Barbara", LastName: "Liskov"}
	if got != want {
		t.Errorf("resolveUser() = %+v, want %+v", got, want)
	}
	if created.Email != "new-hire@example.edu" || created.FirstName != "Barbara" || created.LastName != "Liskov" {
		t.Errorf("CreateUser params = %+v", created)
	}
	if created.PasswordHash != nil {
		t.Error("auto-created SSO user should have no password_hash")
	}
	if created.IsPlatformAdmin {
		t.Error("auto-created SSO user should never be a platform admin")
	}
	if created.SsoIssuer == nil || *created.SsoIssuer != "https://idp.example.edu" {
		t.Errorf("CreateUser SsoIssuer = %v, want https://idp.example.edu", created.SsoIssuer)
	}
	if created.SsoSubject == nil || *created.SsoSubject != "sub-789" {
		t.Errorf("CreateUser SsoSubject = %v, want sub-789", created.SsoSubject)
	}
}

func TestOIDCAuthenticator_ResolveUser_DeactivatedBySSOIdentity(t *testing.T) {
	q := &dbfake.Querier{
		GetUserBySSOIdentityFunc: func(ctx context.Context, arg db.GetUserBySSOIdentityParams) (db.User, error) {
			return db.User{ID: 2, Email: "former@example.edu", DeactivatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}, nil
		},
	}
	authr := &OIDCAuthenticator{queries: q}

	_, err := authr.resolveUser(context.Background(), "https://idp.example.edu", ssoClaims{Subject: "sub-2", Email: "former@example.edu", EmailVerified: true})
	if !errors.Is(err, ErrAccountDeactivated) {
		t.Errorf("resolveUser() error = %v, want ErrAccountDeactivated", err)
	}
}

func TestOIDCAuthenticator_ResolveUser_DeactivatedByEmail(t *testing.T) {
	q := &dbfake.Querier{
		GetUserBySSOIdentityFunc: func(ctx context.Context, arg db.GetUserBySSOIdentityParams) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
		GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
			return db.User{ID: 3, Email: "former2@example.edu", DeactivatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}, nil
		},
		SetUserSSOIdentityFunc: func(ctx context.Context, arg db.SetUserSSOIdentityParams) error {
			t.Fatal("should not link sso identity onto a deactivated account")
			return nil
		},
	}
	authr := &OIDCAuthenticator{queries: q}

	_, err := authr.resolveUser(context.Background(), "https://idp.example.edu", ssoClaims{Subject: "sub-3", Email: "former2@example.edu", EmailVerified: true})
	if !errors.Is(err, ErrAccountDeactivated) {
		t.Errorf("resolveUser() error = %v, want ErrAccountDeactivated", err)
	}
}

func TestSSOClaims_Validate(t *testing.T) {
	tests := []struct {
		name    string
		claims  ssoClaims
		wantErr bool
	}{
		{"valid", ssoClaims{Subject: "sub-1", Email: "a@example.edu", EmailVerified: true}, false},
		{"missing subject", ssoClaims{Email: "a@example.edu", EmailVerified: true}, true},
		{"missing email", ssoClaims{Subject: "sub-1", EmailVerified: true}, true},
		{"email not verified", ssoClaims{Subject: "sub-1", Email: "a@example.edu", EmailVerified: false}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.claims.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSSOClaims_Names(t *testing.T) {
	tests := []struct {
		name      string
		claims    ssoClaims
		wantFirst string
		wantLast  string
	}{
		{"given+family present", ssoClaims{GivenName: "Ada", FamilyName: "Lovelace", Name: "ignored"}, "Ada", "Lovelace"},
		{"only display name, two words", ssoClaims{Name: "Grace Hopper"}, "Grace", "Hopper"},
		{"only display name, three words", ssoClaims{Name: "Mary Jane Watson"}, "Mary", "Jane Watson"},
		{"only display name, one word", ssoClaims{Name: "Cher"}, "Cher", ""},
		{"nothing at all", ssoClaims{}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, last := tt.claims.names()
			if first != tt.wantFirst || last != tt.wantLast {
				t.Errorf("names() = (%q, %q), want (%q, %q)", first, last, tt.wantFirst, tt.wantLast)
			}
		})
	}
}
