package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/auth"
	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
	"github.com/ggem/coglab-manager-go/internal/mcdi/mcdifake"
)

// sessionCookieNameForTest mirrors the unexported cookie name
// auth.SessionManager uses -- there's no exported constant to reference
// from outside the auth package.
const sessionCookieNameForTest = "coglab_session"

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestServer(q *dbfake.Querier) *Server {
	return newTestServerWithMCDI(q, &mcdifake.Client{})
}

// newTestServerWithMCDI is newTestServer for the handful of tests that
// need to control the fake MCDI client's behavior (e.g. configuring it
// to fail) rather than accepting the default no-op one.
func newTestServerWithMCDI(q *dbfake.Querier, mcdiClient *mcdifake.Client) *Server {
	return NewServer(auth.NewPasswordAuthenticator(q), auth.NewSessionManager(q, false), audit.NewRecorder(q), q, mcdiClient, discardLogger())
}

func postJSON(t *testing.T, s *Server, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	return rec
}

func TestHandleLogin_Success(t *testing.T) {
	hash, err := auth.HashPassword("s3cret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	var capturedSession db.CreateSessionParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
			return db.User{ID: 1, Email: "researcher@example.edu", FirstName: "Ada", LastName: "Lovelace", PasswordHash: &hash}, nil
		},
		CreateSessionFunc: func(ctx context.Context, arg db.CreateSessionParams) (db.Session, error) {
			capturedSession = arg
			return db.Session{ID: 42, UserID: arg.UserID, TokenHash: arg.TokenHash}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s := newTestServer(q)

	rec := postJSON(t, s, "/login", loginRequest{Email: "researcher@example.edu", Password: "s3cret"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	var got loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.User.ID != 1 || got.User.Email != "researcher@example.edu" {
		t.Errorf("response user = %+v, want ID=1 Email=researcher@example.edu", got.User)
	}
	if len(rec.Result().Cookies()) != 1 {
		t.Errorf("expected a session cookie to be set")
	}
	if capturedSession.UserID != 1 {
		t.Errorf("CreateSession UserID = %d, want 1", capturedSession.UserID)
	}
	if capturedAudit.Action != auth.ActionLoginSucceeded {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, auth.ActionLoginSucceeded)
	}
	if capturedAudit.ActorUserID == nil || *capturedAudit.ActorUserID != 1 {
		t.Errorf("audit ActorUserID = %v, want 1", capturedAudit.ActorUserID)
	}
}

func TestHandleLogin_InvalidCredentials(t *testing.T) {
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s := newTestServer(q)

	rec := postJSON(t, s, "/login", loginRequest{Email: "nobody@example.edu", Password: "whatever"})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if capturedAudit.Action != auth.ActionLoginFailed {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, auth.ActionLoginFailed)
	}
	if capturedAudit.ActorUserID != nil {
		t.Errorf("audit ActorUserID = %v, want nil for an unauthenticated event", capturedAudit.ActorUserID)
	}
	var metadata map[string]string
	if err := json.Unmarshal(capturedAudit.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal audit metadata: %v", err)
	}
	if metadata["reason"] != "invalid_credentials" {
		t.Errorf("audit metadata reason = %q, want invalid_credentials", metadata["reason"])
	}
}

func TestHandleLogin_MissingFields(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := postJSON(t, s, "/login", loginRequest{Email: "researcher@example.edu"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleLogout_Success(t *testing.T) {
	var revokedHash []byte
	var touched bool
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		GetSessionByTokenHashFunc: func(ctx context.Context, h []byte) (db.Session, error) {
			return db.Session{ID: 42, UserID: 7, TokenHash: h}, nil
		},
		TouchSessionLastSeenFunc: func(ctx context.Context, sessionID int64) error {
			touched = true
			return nil
		},
		RevokeSessionFunc: func(ctx context.Context, h []byte) error {
			revokedHash = h
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s := newTestServer(q)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieNameForTest, Value: "sometoken"})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if !touched {
		t.Error("expected requireAuth to touch the session's last_seen_at")
	}
	if revokedHash == nil {
		t.Error("expected RevokeSession to be called")
	}
	if capturedAudit.Action != auth.ActionLogout {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, auth.ActionLogout)
	}
	if capturedAudit.ActorUserID == nil || *capturedAudit.ActorUserID != 7 {
		t.Errorf("audit ActorUserID = %v, want 7", capturedAudit.ActorUserID)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Error("expected logout to clear the session cookie")
	}
}

func TestHandleLogout_NoSession(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
