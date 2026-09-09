package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/auth"
	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

// fakeSSOAuthenticator is a test double for auth.SSOAuthenticator -- it lets
// handler tests exercise handleSSOLogin/handleSSOCallback without a live
// Dex/OIDC provider to discover against or a real ID token to verify.
type fakeSSOAuthenticator struct {
	authCodeURL      string
	gotState         string
	gotNonce         string
	exchangeIdentity auth.Identity
	exchangeErr      error
	gotCode          string
	gotExpectedNonce string
}

func (f *fakeSSOAuthenticator) AuthCodeURL(state, nonce string) string {
	f.gotState = state
	f.gotNonce = nonce
	return f.authCodeURL
}

func (f *fakeSSOAuthenticator) Exchange(ctx context.Context, code, expectedNonce string) (auth.Identity, error) {
	f.gotCode = code
	f.gotExpectedNonce = expectedNonce
	return f.exchangeIdentity, f.exchangeErr
}

func newSSOTestServer(q *dbfake.Querier, oidc auth.SSOAuthenticator) *Server {
	return NewServer(auth.NewPasswordAuthenticator(q), auth.NewSessionManager(q, false), audit.NewRecorder(q), q, nil, nil, discardLogger(), oidc)
}

func TestHandleSSOConfig_Disabled(t *testing.T) {
	s := newSSOTestServer(&dbfake.Querier{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth/sso/config", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != `{"enabled":false}`+"\n" {
		t.Errorf("body = %q, want enabled:false", got)
	}
}

func TestHandleSSOConfig_Enabled(t *testing.T) {
	s := newSSOTestServer(&dbfake.Querier{}, &fakeSSOAuthenticator{})

	req := httptest.NewRequest(http.MethodGet, "/auth/sso/config", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != `{"enabled":true}`+"\n" {
		t.Errorf("body = %q, want enabled:true", got)
	}
}

func TestHandleSSOLogin_RedirectsAndSetsCookies(t *testing.T) {
	fake := &fakeSSOAuthenticator{authCodeURL: "https://idp.example.edu/auth?foo=bar"}
	s := newSSOTestServer(&dbfake.Querier{}, fake)

	req := httptest.NewRequest(http.MethodGet, "/auth/sso/login", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != fake.authCodeURL {
		t.Errorf("Location = %q, want %q", got, fake.authCodeURL)
	}
	if fake.gotState == "" || fake.gotNonce == "" {
		t.Fatalf("AuthCodeURL called with empty state/nonce: state=%q nonce=%q", fake.gotState, fake.gotNonce)
	}

	cookies := rec.Result().Cookies()
	var state, nonce *http.Cookie
	for _, c := range cookies {
		switch c.Name {
		case oidcStateCookie:
			state = c
		case oidcNonceCookie:
			nonce = c
		}
	}
	if state == nil || state.Value != fake.gotState {
		t.Errorf("state cookie = %+v, want value %q", state, fake.gotState)
	}
	if nonce == nil || nonce.Value != fake.gotNonce {
		t.Errorf("nonce cookie = %+v, want value %q", nonce, fake.gotNonce)
	}
	for _, c := range []*http.Cookie{state, nonce} {
		if !c.HttpOnly {
			t.Errorf("cookie %s: HttpOnly = false, want true", c.Name)
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Errorf("cookie %s: SameSite = %v, want Lax", c.Name, c.SameSite)
		}
		if c.Path != "/auth/sso" {
			t.Errorf("cookie %s: Path = %q, want /auth/sso", c.Name, c.Path)
		}
	}
}

func TestHandleSSOLogin_NotRegisteredWhenSSODisabled(t *testing.T) {
	s := newSSOTestServer(&dbfake.Querier{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth/sso/login", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (route shouldn't exist when SSO is unconfigured)", rec.Code, http.StatusNotFound)
	}
}

func callbackRequest(state, nonce, queryState, code string) *http.Request {
	url := "/auth/sso/callback?state=" + queryState
	if code != "" {
		url += "&code=" + code
	}
	req := httptest.NewRequest(http.MethodGet, url, nil)
	if state != "" {
		req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: state})
	}
	if nonce != "" {
		req.AddCookie(&http.Cookie{Name: oidcNonceCookie, Value: nonce})
	}
	return req
}

func TestHandleSSOCallback_MissingStateCookie_Rejected(t *testing.T) {
	fake := &fakeSSOAuthenticator{}
	s := newSSOTestServer(&dbfake.Querier{CreateAuditEventFunc: noopCreateAuditEvent}, fake)

	req := callbackRequest("", "nonce-1", "state-1", "code-1")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/?sso_error=1" {
		t.Fatalf("status/location = %d %q, want 302 to /?sso_error=1", rec.Code, rec.Header().Get("Location"))
	}
	if fake.gotCode != "" {
		t.Error("Exchange should not be called when the state cookie is missing")
	}
}

func TestHandleSSOCallback_StateMismatch_Rejected(t *testing.T) {
	fake := &fakeSSOAuthenticator{}
	s := newSSOTestServer(&dbfake.Querier{CreateAuditEventFunc: noopCreateAuditEvent}, fake)

	req := callbackRequest("cookie-state", "cookie-nonce", "different-state", "code-1")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/?sso_error=1" {
		t.Fatalf("status/location = %d %q, want 302 to /?sso_error=1", rec.Code, rec.Header().Get("Location"))
	}
	if fake.gotCode != "" {
		t.Error("Exchange should not be called on state mismatch")
	}
}

func TestHandleSSOCallback_MissingCode_Rejected(t *testing.T) {
	fake := &fakeSSOAuthenticator{}
	s := newSSOTestServer(&dbfake.Querier{CreateAuditEventFunc: noopCreateAuditEvent}, fake)

	req := callbackRequest("state-1", "nonce-1", "state-1", "")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/?sso_error=1" {
		t.Fatalf("status/location = %d %q, want 302 to /?sso_error=1", rec.Code, rec.Header().Get("Location"))
	}
	if fake.gotCode != "" {
		t.Error("Exchange should not be called when code is missing")
	}
}

func TestHandleSSOCallback_ExchangeFails_Rejected(t *testing.T) {
	fake := &fakeSSOAuthenticator{exchangeErr: auth.ErrAccountDeactivated}
	s := newSSOTestServer(&dbfake.Querier{CreateAuditEventFunc: noopCreateAuditEvent}, fake)

	req := callbackRequest("state-1", "nonce-1", "state-1", "code-1")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/?sso_error=1" {
		t.Fatalf("status/location = %d %q, want 302 to /?sso_error=1", rec.Code, rec.Header().Get("Location"))
	}
}

func TestHandleSSOCallback_Success(t *testing.T) {
	var capturedSession db.CreateSessionParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateSessionFunc: func(ctx context.Context, arg db.CreateSessionParams) (db.Session, error) {
			capturedSession = arg
			return db.Session{ID: 1, UserID: arg.UserID, TokenHash: arg.TokenHash}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	fake := &fakeSSOAuthenticator{
		exchangeIdentity: auth.Identity{UserID: 42, Email: "ada@example.edu", FirstName: "Ada", LastName: "Lovelace"},
	}
	s := newSSOTestServer(q, fake)

	req := callbackRequest("state-1", "nonce-1", "state-1", "code-1")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("status/location = %d %q, want 302 to /", rec.Code, rec.Header().Get("Location"))
	}
	if fake.gotCode != "code-1" || fake.gotExpectedNonce != "nonce-1" {
		t.Errorf("Exchange called with code=%q nonce=%q, want code-1/nonce-1", fake.gotCode, fake.gotExpectedNonce)
	}
	if capturedSession.UserID != 42 {
		t.Errorf("CreateSession UserID = %d, want 42", capturedSession.UserID)
	}
	if capturedAudit.Action != auth.ActionLoginSucceeded {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, auth.ActionLoginSucceeded)
	}

	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieNameForTest {
			found = true
		}
	}
	if !found {
		t.Error("expected a session cookie to be set on successful SSO login")
	}
}

func noopCreateAuditEvent(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
	return db.AuditEvent{ID: 1}, nil
}
