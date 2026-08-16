package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestSessionManager_IssueAndValidate(t *testing.T) {
	var capturedParams db.CreateSessionParams
	var storedSession db.Session

	q := &dbfake.Querier{
		CreateSessionFunc: func(ctx context.Context, arg db.CreateSessionParams) (db.Session, error) {
			capturedParams = arg
			storedSession = db.Session{ID: 42, TokenHash: arg.TokenHash, UserID: arg.UserID, ExpiresAt: arg.ExpiresAt}
			return storedSession, nil
		},
		GetSessionByTokenHashFunc: func(ctx context.Context, tokenHash []byte) (db.Session, error) {
			if string(tokenHash) != string(storedSession.TokenHash) {
				t.Fatalf("GetSessionByTokenHash called with unexpected hash")
			}
			return storedSession, nil
		},
	}
	mgr := NewSessionManager(q, false)

	rec := httptest.NewRecorder()
	issueReq := httptest.NewRequest(http.MethodPost, "/login", nil)
	if err := mgr.Issue(context.Background(), rec, issueReq, 7); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if capturedParams.UserID != 7 {
		t.Errorf("CreateSession UserID = %d, want 7", capturedParams.UserID)
	}
	if len(capturedParams.TokenHash) != 32 {
		t.Errorf("TokenHash length = %d, want 32 (sha256)", len(capturedParams.TokenHash))
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != sessionCookieName {
		t.Errorf("cookie name = %q, want %q", cookie.Name, sessionCookieName)
	}
	if !cookie.HttpOnly {
		t.Error("cookie should be HttpOnly")
	}
	if cookie.Secure {
		t.Error("cookie should not be Secure when secureCookies=false")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	}

	validateReq := httptest.NewRequest(http.MethodGet, "/", nil)
	validateReq.AddCookie(cookie)

	got, err := mgr.Validate(context.Background(), validateReq)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got.ID != storedSession.ID {
		t.Errorf("Validate returned session ID %d, want %d", got.ID, storedSession.ID)
	}
}

func TestSessionManager_Issue_SecureCookieWhenConfigured(t *testing.T) {
	q := &dbfake.Querier{
		CreateSessionFunc: func(ctx context.Context, arg db.CreateSessionParams) (db.Session, error) {
			return db.Session{ID: 1, TokenHash: arg.TokenHash, UserID: arg.UserID}, nil
		},
	}
	mgr := NewSessionManager(q, true)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	if err := mgr.Issue(context.Background(), rec, req, 1); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure {
		t.Error("cookie should be Secure when secureCookies=true")
	}
}

func TestSessionManager_Validate_NoCookie(t *testing.T) {
	mgr := NewSessionManager(&dbfake.Querier{}, false)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	if _, err := mgr.Validate(context.Background(), req); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Validate() error = %v, want ErrSessionNotFound", err)
	}
}

func TestSessionManager_Validate_UnknownToken(t *testing.T) {
	q := &dbfake.Querier{
		GetSessionByTokenHashFunc: func(ctx context.Context, tokenHash []byte) (db.Session, error) {
			return db.Session{}, pgx.ErrNoRows
		},
	}
	mgr := NewSessionManager(q, false)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "bogus"})

	if _, err := mgr.Validate(context.Background(), req); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Validate() error = %v, want ErrSessionNotFound", err)
	}
}

func TestSessionManager_Validate_Revoked(t *testing.T) {
	q := &dbfake.Querier{
		GetSessionByTokenHashFunc: func(ctx context.Context, tokenHash []byte) (db.Session, error) {
			return db.Session{RevokedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}, nil
		},
	}
	mgr := NewSessionManager(q, false)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "whatever"})

	if _, err := mgr.Validate(context.Background(), req); !errors.Is(err, ErrSessionRevoked) {
		t.Errorf("Validate() error = %v, want ErrSessionRevoked", err)
	}
}

func TestSessionManager_Validate_Expired(t *testing.T) {
	q := &dbfake.Querier{
		GetSessionByTokenHashFunc: func(ctx context.Context, tokenHash []byte) (db.Session, error) {
			return db.Session{ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true}}, nil
		},
	}
	mgr := NewSessionManager(q, false)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "whatever"})

	if _, err := mgr.Validate(context.Background(), req); !errors.Is(err, ErrSessionExpired) {
		t.Errorf("Validate() error = %v, want ErrSessionExpired", err)
	}
}

func TestSessionManager_Revoke(t *testing.T) {
	var revokedHash []byte
	q := &dbfake.Querier{
		RevokeSessionFunc: func(ctx context.Context, tokenHash []byte) error {
			revokedHash = tokenHash
			return nil
		},
	}
	mgr := NewSessionManager(q, false)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "sometoken"})
	rec := httptest.NewRecorder()

	if err := mgr.Revoke(context.Background(), rec, req); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	if revokedHash == nil {
		t.Error("RevokeSession was not called")
	}
	if string(revokedHash) != string(hashToken("sometoken")) {
		t.Error("RevokeSession was called with the wrong token hash")
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie in response, got %d", len(cookies))
	}
	if cookies[0].MaxAge >= 0 {
		t.Errorf("clearing cookie should set MaxAge < 0, got %d", cookies[0].MaxAge)
	}
}

func TestSessionManager_Revoke_NoCookie(t *testing.T) {
	mgr := NewSessionManager(&dbfake.Querier{}, false)
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()

	if err := mgr.Revoke(context.Background(), rec, req); err != nil {
		t.Fatalf("Revoke with no cookie should not error: %v", err)
	}
}
