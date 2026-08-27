//go:build integration

// This test drives the login/logout endpoints end to end against a real
// Postgres instance (via internal/dbtest), exercising the same wiring
// cmd/api uses in production: real db.Queries feeding a real
// PasswordAuthenticator, SessionManager, and audit.Recorder. Run with:
// go test -tags=integration ./...
package httpapi

import (
	"bytes"
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

	s := NewServer(auth.NewPasswordAuthenticator(testQueries), auth.NewSessionManager(testQueries, false), audit.NewRecorder(testQueries), testQueries, discardLogger())

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

// TestExperimentsFlow_Integration exercises the experiments domain's HTTP
// layer against real Postgres: unlike the dbfake-backed unit tests, this
// catches mistakes in the generated SQL and route wiring that a fake
// Querier can't. There's no /labs endpoint yet (see server.go's comment on
// the lab-scoped routes), so the lab row is inserted directly.
func TestExperimentsFlow_Integration(t *testing.T) {
	ctx := context.Background()

	var labID int64
	if err := testPool.QueryRow(ctx, "insert into labs (name, short_name) values ($1, $2) returning id",
		"Integration Test Lab", fmt.Sprintf("itl-%d", time.Now().UnixNano())).Scan(&labID); err != nil {
		t.Fatalf("insert lab: %v", err)
	}

	hash, err := auth.HashPassword("s3cret-integration-test")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	email := fmt.Sprintf("integration-experiments-%d@example.edu", time.Now().UnixNano())
	if _, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email: email, FirstName: "Integration", LastName: "Test", PasswordHash: &hash,
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	s := NewServer(auth.NewPasswordAuthenticator(testQueries), auth.NewSessionManager(testQueries, false), audit.NewRecorder(testQueries), testQueries, discardLogger())

	loginRec := postJSON(t, s, "/login", loginRequest{Email: email, Password: "s3cret-integration-test"})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d; body = %s", loginRec.Code, http.StatusOK, loginRec.Body)
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie after login, got %d", len(cookies))
	}
	cookie := cookies[0]

	do := func(method, path string, body any) *httptest.ResponseRecorder {
		var r *bytes.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}
			r = bytes.NewReader(b)
		} else {
			r = bytes.NewReader(nil)
		}
		req := httptest.NewRequest(method, path, r)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		s.Routes().ServeHTTP(rec, req)
		return rec
	}
	decode := func(rec *httptest.ResponseRecorder, v any) {
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("unmarshal response body: %v; body = %s", err, rec.Body)
		}
	}

	conditionRec := do(http.MethodPost, fmt.Sprintf("/labs/%d/conditions/", labID), conditionRequest{Name: "Stimulus Type"})
	if conditionRec.Code != http.StatusCreated {
		t.Fatalf("create condition status = %d, want %d; body = %s", conditionRec.Code, http.StatusCreated, conditionRec.Body)
	}
	var condition conditionResponse
	decode(conditionRec, &condition)

	equipmentRec := do(http.MethodPost, fmt.Sprintf("/labs/%d/equipment/", labID), equipmentRequest{Name: "Eye Tracker", Quantity: 1})
	if equipmentRec.Code != http.StatusCreated {
		t.Fatalf("create equipment status = %d, want %d; body = %s", equipmentRec.Code, http.StatusCreated, equipmentRec.Body)
	}
	var equipment equipmentResponse
	decode(equipmentRec, &equipment)

	roleRec := do(http.MethodPost, fmt.Sprintf("/labs/%d/experiment-roles/", labID), experimentRoleRequest{Name: "Experimenter"})
	if roleRec.Code != http.StatusCreated {
		t.Fatalf("create experiment role status = %d, want %d; body = %s", roleRec.Code, http.StatusCreated, roleRec.Body)
	}
	var role experimentRoleResponse
	decode(roleRec, &role)

	minAge, maxAge := 6.0, 18.0
	startDate := "2026-01-01"
	experimentRec := do(http.MethodPost, fmt.Sprintf("/labs/%d/experiments/", labID), experimentRequest{
		Name: "Looking Time Study", Status: "not_run", Sessions: 1, DurationMinutes: 30,
		AgeRangeMinMonths: &minAge, AgeRangeMaxMonths: &maxAge, StartDate: &startDate,
	})
	if experimentRec.Code != http.StatusCreated {
		t.Fatalf("create experiment status = %d, want %d; body = %s", experimentRec.Code, http.StatusCreated, experimentRec.Body)
	}
	var experiment experimentResponse
	decode(experimentRec, &experiment)
	if experiment.LabID != labID {
		t.Errorf("experiment LabID = %d, want %d", experiment.LabID, labID)
	}

	if rec := do(http.MethodPost, fmt.Sprintf("/experiments/%d/conditions/", experiment.ID), addConditionRequest{ConditionID: condition.ID}); rec.Code != http.StatusNoContent {
		t.Fatalf("add experiment condition status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if rec := do(http.MethodPost, fmt.Sprintf("/experiments/%d/equipment/", experiment.ID), addEquipmentRequest{EquipmentID: equipment.ID}); rec.Code != http.StatusNoContent {
		t.Fatalf("add experiment equipment status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if rec := do(http.MethodPost, fmt.Sprintf("/experiments/%d/training-requirements/", experiment.ID), addTrainingRequirementRequest{ExperimentRoleID: role.ID}); rec.Code != http.StatusNoContent {
		t.Fatalf("add training requirement status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}

	var conditions []conditionResponse
	decode(do(http.MethodGet, fmt.Sprintf("/experiments/%d/conditions/", experiment.ID), nil), &conditions)
	if len(conditions) != 1 || conditions[0].ID != condition.ID {
		t.Errorf("experiment conditions = %+v, want [%d]", conditions, condition.ID)
	}

	if rec := do(http.MethodDelete, fmt.Sprintf("/experiments/%d/conditions/%d", experiment.ID, condition.ID), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("remove experiment condition status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	decode(do(http.MethodGet, fmt.Sprintf("/experiments/%d/conditions/", experiment.ID), nil), &conditions)
	if len(conditions) != 0 {
		t.Errorf("experiment conditions after removal = %+v, want none", conditions)
	}

	updateRec := do(http.MethodPut, fmt.Sprintf("/experiments/%d/", experiment.ID), experimentRequest{
		Name: "Looking Time Study (Revised)", Status: "pilot", Sessions: 1, DurationMinutes: 30,
		AgeRangeMinMonths: &minAge, AgeRangeMaxMonths: &maxAge, StartDate: &startDate,
	})
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update experiment status = %d, want %d; body = %s", updateRec.Code, http.StatusOK, updateRec.Body)
	}
	var updated experimentResponse
	decode(updateRec, &updated)
	if updated.Status != "pilot" {
		t.Errorf("updated experiment Status = %q, want %q", updated.Status, "pilot")
	}

	if rec := do(http.MethodPost, fmt.Sprintf("/experiments/%d/deactivate", experiment.ID), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("deactivate experiment status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	var afterDeactivate experimentResponse
	decode(do(http.MethodGet, fmt.Sprintf("/experiments/%d/", experiment.ID), nil), &afterDeactivate)
	if !afterDeactivate.Deactivated {
		t.Errorf("experiment Deactivated = false after deactivation")
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
