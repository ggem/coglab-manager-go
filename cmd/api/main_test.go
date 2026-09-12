package main

import (
	"context"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/auth"
)

// TestNewOIDCAuthenticator_UnsetReturnsTrueNilInterface guards against a
// real bug this deployment surfaced: newOIDCAuthenticator used to return
// the concrete *auth.OIDCAuthenticator type, so `return nil, nil` produced
// a typed-nil pointer. Assigning that to Server.oidc (an interface field)
// makes `s.oidc != nil` true even with SSO totally unconfigured -- the
// classic Go nil-pointer-in-non-nil-interface trap. handleSSOConfig then
// reports SSO as enabled, and server.go registers the /auth/sso/* routes,
// for a deployment that set none of the OIDC_* env vars.
func TestNewOIDCAuthenticator_UnsetReturnsTrueNilInterface(t *testing.T) {
	// Explicit empties rather than relying on ambient environment --
	// this guards against exactly one deployment leaving these unset.
	t.Setenv("OIDC_ISSUER_URL", "")
	t.Setenv("OIDC_CLIENT_ID", "")
	t.Setenv("OIDC_CLIENT_SECRET", "")
	t.Setenv("OIDC_REDIRECT_URL", "")

	got, err := newOIDCAuthenticator(context.Background(), nil)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	// The bug lives specifically at an interface assignment boundary --
	// comparing `got` to nil using its own declared return type wouldn't
	// reproduce it even with the old buggy signature (*auth.OIDCAuthenticator),
	// since a concrete nil pointer compares equal to nil directly. Forcing
	// the same conversion NewServer's `oidc auth.SSOAuthenticator` parameter
	// performs is what actually exercises the trap.
	var sso auth.SSOAuthenticator = got
	if sso != nil {
		t.Fatalf("sso = %#v, want a true nil interface -- sso != nil means Server.oidc would wrongly report SSO as configured", sso)
	}
}

func TestNewOIDCAuthenticator_PartialConfigErrors(t *testing.T) {
	t.Setenv("OIDC_ISSUER_URL", "https://idp.example.edu")
	t.Setenv("OIDC_CLIENT_ID", "")
	t.Setenv("OIDC_CLIENT_SECRET", "")
	t.Setenv("OIDC_REDIRECT_URL", "")

	got, err := newOIDCAuthenticator(context.Background(), nil)
	if err == nil {
		t.Fatal("err = nil, want an error for a partially-set OIDC config")
	}
	if got != nil {
		t.Fatalf("got = %#v, want nil on error", got)
	}
}

// TestRequireTZ_Unset guards against a real bug this deployment
// surfaced: the scheduler compares process-local "now" directly against
// naive schedule_date/schedule_time_start columns holding the lab's own
// local wall-clock digits. A container with no TZ set defaults to UTC,
// silently misclassifying genuinely-future appointments as already
// past -- this had always been masked in local dev by the developer's
// own machine already being in the lab's timezone.
func TestRequireTZ_Unset(t *testing.T) {
	t.Setenv("TZ", "")

	if err := requireTZ(); err == nil {
		t.Fatal("requireTZ() = nil, want an error when TZ is unset")
	}
}

// TestRequireTZ_Invalid guards a gap the presence-only check left open:
// a typo'd or nonexistent zone name (still non-empty) passed the old
// check but would silently resolve to UTC at runtime the same way an
// entirely-unset TZ does, since Go's own TZ lookup falls back to UTC on
// an unrecognized name.
func TestRequireTZ_Invalid(t *testing.T) {
	t.Setenv("TZ", "Not/A_Real_Zone")

	if err := requireTZ(); err == nil {
		t.Fatal("requireTZ() = nil, want an error for a nonexistent zone name")
	}
}

func TestRequireTZ_Set(t *testing.T) {
	t.Setenv("TZ", "America/Denver")

	if err := requireTZ(); err != nil {
		t.Errorf("requireTZ() = %v, want nil when TZ is set", err)
	}
}
