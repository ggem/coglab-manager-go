//go:build integration

// This test drives the login/logout endpoints end to end against a real
// Postgres instance (via internal/dbtest), exercising the same wiring
// cmd/api uses in production: real db.Queries feeding a real
// PasswordAuthenticator, SessionManager, and audit.Recorder. Run with:
// go test -tags=integration ./...
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/auth"
	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/dbtest"
)

var (
	testQueries *db.Queries
	testPool    *pgxpool.Pool
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	pg, err := dbtest.StartPostgres(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start postgres:", err)
		os.Exit(1)
	}
	testQueries = db.New(pg.Pool)
	testPool = pg.Pool

	code := m.Run()

	if err := pg.Close(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "close postgres:", err)
	}
	os.Exit(code)
}

func TestLoginLogoutFlow_Integration(t *testing.T) {
	ctx := context.Background()

	hash, err := auth.HashPassword("s3cret-integration-test")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	email := fmt.Sprintf("integration-%d@example.edu", time.Now().UnixNano())
	user, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email:        email,
		FirstName:    "Integration",
		LastName:     "Test",
		PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	s := NewServer(auth.NewPasswordAuthenticator(testQueries), auth.NewSessionManager(testQueries, false), audit.NewRecorder(testQueries), discardLogger())

	loginRec := postJSON(t, s, "/login", loginRequest{Email: email, Password: "s3cret-integration-test"})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d; body = %s", loginRec.Code, http.StatusOK, loginRec.Body)
	}
	var loginResp loginResponse
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}
	if loginResp.User.ID != user.ID {
		t.Errorf("logged-in user ID = %d, want %d", loginResp.User.ID, user.ID)
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie after login, got %d", len(cookies))
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logoutReq.AddCookie(cookies[0])
	logoutRec := httptest.NewRecorder()
	s.Routes().ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d; body = %s", logoutRec.Code, http.StatusNoContent, logoutRec.Body)
	}

	// The session should now be rejected: a second logout with the same
	// (now-revoked) cookie must fail authentication.
	replayReq := httptest.NewRequest(http.MethodPost, "/logout", nil)
	replayReq.AddCookie(cookies[0])
	replayRec := httptest.NewRecorder()
	s.Routes().ServeHTTP(replayRec, replayReq)
	if replayRec.Code != http.StatusUnauthorized {
		t.Errorf("logout with a revoked cookie: status = %d, want %d", replayRec.Code, http.StatusUnauthorized)
	}

	events, err := auditEventsForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("query audit events: %v", err)
	}
	wantActions := []string{auth.ActionLoginSucceeded, auth.ActionLogout}
	if len(events) != len(wantActions) {
		t.Fatalf("audit events for user = %v, want actions %v", events, wantActions)
	}
	for i, want := range wantActions {
		if events[i] != want {
			t.Errorf("audit event[%d] = %q, want %q", i, events[i], want)
		}
	}
}

func auditEventsForUser(ctx context.Context, userID int64) ([]string, error) {
	rows, err := testPool.Query(ctx, "select action from audit_events where actor_user_id = $1 order by id", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []string
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, rows.Err()
}
