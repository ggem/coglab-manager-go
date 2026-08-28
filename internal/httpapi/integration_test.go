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
	user, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email: email, FirstName: "Integration", LastName: "Test", PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// The experiments domain is lab-scoped: every request checks the user
	// is a lab_memberships row for the target lab (see lab_authz.go). No
	// CreateRole query exists yet (roles are the separate lab-membership
	// permission-level concept, out of scope so far), so both the role and
	// the membership are inserted directly.
	var roleID int64
	if err := testPool.QueryRow(ctx, "insert into roles (name, description) values ($1, $2) returning id",
		fmt.Sprintf("integration-test-role-%d", time.Now().UnixNano()), "integration test role").Scan(&roleID); err != nil {
		t.Fatalf("insert role: %v", err)
	}
	if _, err := testPool.Exec(ctx, "insert into lab_memberships (user_id, lab_id, role_id) values ($1, $2, $3)", user.ID, labID, roleID); err != nil {
		t.Fatalf("insert lab_membership: %v", err)
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

	// Lab-membership authorization against the real query, not just the
	// dbfake stub the unit tests use: a lab this user has no
	// lab_memberships row for must be unreachable, both to create under
	// and to reach an existing resource through.
	var otherLabID int64
	if err := testPool.QueryRow(ctx, "insert into labs (name, short_name) values ($1, $2) returning id",
		"Other Lab", fmt.Sprintf("other-%d", time.Now().UnixNano())).Scan(&otherLabID); err != nil {
		t.Fatalf("insert other lab: %v", err)
	}
	if rec := do(http.MethodPost, fmt.Sprintf("/labs/%d/conditions/", otherLabID), conditionRequest{Name: "Should Be Blocked"}); rec.Code != http.StatusNotFound {
		t.Errorf("create condition in a lab this user isn't a member of: status = %d, want %d; body = %s", rec.Code, http.StatusNotFound, rec.Body)
	}
	if rec := do(http.MethodGet, fmt.Sprintf("/experiments/%d/", experiment.ID), nil); rec.Code != http.StatusOK {
		t.Errorf("sanity check: own lab's experiment should still be reachable, status = %d", rec.Code)
	}
}

// TestSchedulingFlow_Integration exercises M5's real point: that the
// ported two-phase search (internal/scheduling) wired to real SQL data
// actually finds a correct, consistent staff assignment and commits it --
// not just that the endpoints respond, which the dbfake-backed unit tests
// already cover extensively.
func TestSchedulingFlow_Integration(t *testing.T) {
	ctx := context.Background()

	var labID int64
	if err := testPool.QueryRow(ctx, "insert into labs (name, short_name) values ($1, $2) returning id",
		"Scheduling Test Lab", fmt.Sprintf("stl-%d", time.Now().UnixNano())).Scan(&labID); err != nil {
		t.Fatalf("insert lab: %v", err)
	}

	hash, err := auth.HashPassword("s3cret-integration-test")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	actor, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email:     fmt.Sprintf("scheduling-actor-%d@example.edu", time.Now().UnixNano()),
		FirstName: "Actor", LastName: "Test", PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateUser(actor): %v", err)
	}
	experimenter, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email:     fmt.Sprintf("scheduling-experimenter-%d@example.edu", time.Now().UnixNano()),
		FirstName: "Experimenter", LastName: "Candidate",
	})
	if err != nil {
		t.Fatalf("CreateUser(experimenter): %v", err)
	}
	sitter, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email:     fmt.Sprintf("scheduling-sitter-%d@example.edu", time.Now().UnixNano()),
		FirstName: "Sitter", LastName: "Candidate",
	})
	if err != nil {
		t.Fatalf("CreateUser(sitter): %v", err)
	}

	var roleID int64
	if err := testPool.QueryRow(ctx, "insert into roles (name, description) values ($1, $2) returning id",
		fmt.Sprintf("scheduling-test-role-%d", time.Now().UnixNano()), "integration test role").Scan(&roleID); err != nil {
		t.Fatalf("insert role: %v", err)
	}
	if _, err := testPool.Exec(ctx, "insert into lab_memberships (user_id, lab_id, role_id) values ($1, $2, $3)", actor.ID, labID, roleID); err != nil {
		t.Fatalf("insert lab_membership: %v", err)
	}

	s := NewServer(auth.NewPasswordAuthenticator(testQueries), auth.NewSessionManager(testQueries, false), audit.NewRecorder(testQueries), testQueries, discardLogger())

	loginRec := postJSON(t, s, "/login", loginRequest{Email: actor.Email, Password: "s3cret-integration-test"})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d; body = %s", loginRec.Code, http.StatusOK, loginRec.Body)
	}
	cookie := loginRec.Result().Cookies()[0]

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

	// Two roles: Experimenter (a real training requirement) and Sitter
	// (designated via set-sitter, not a training requirement -- its
	// requirement is driven by sibling_coming instead, per the plan).
	var experimenterRole, sitterRole experimentRoleResponse
	decode(do(http.MethodPost, fmt.Sprintf("/labs/%d/experiment-roles/", labID), experimentRoleRequest{Name: "Experimenter"}), &experimenterRole)
	decode(do(http.MethodPost, fmt.Sprintf("/labs/%d/experiment-roles/", labID), experimentRoleRequest{Name: "Sitter"}), &sitterRole)
	if rec := do(http.MethodPost, fmt.Sprintf("/experiment-roles/%d/set-sitter", sitterRole.ID), setExperimentRoleSitterRequest{IsSitterRole: true}); rec.Code != http.StatusOK {
		t.Fatalf("set-sitter status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}

	if rec := do(http.MethodPost, fmt.Sprintf("/experiment-roles/%d/trainings/", experimenterRole.ID), addLabMemberTrainingRequest{UserID: experimenter.ID}); rec.Code != http.StatusNoContent {
		t.Fatalf("add experimenter training status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if rec := do(http.MethodPost, fmt.Sprintf("/experiment-roles/%d/trainings/", sitterRole.ID), addLabMemberTrainingRequest{UserID: sitter.ID}); rec.Code != http.StatusNoContent {
		t.Fatalf("add sitter training status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}

	// Both candidates are declared available all day on today's weekday --
	// the search below runs against today's date, so its weekday must
	// match whatever's declared here.
	targetDate := time.Now().UTC().Truncate(24 * time.Hour)
	weekday := int16(targetDate.Weekday())
	for _, userID := range []int64{experimenter.ID, sitter.ID} {
		if _, err := testPool.Exec(ctx,
			"insert into lab_availability_general (user_id, lab_id, weekday, start_time, end_time) values ($1,$2,$3,$4,$5)",
			userID, labID, weekday, "08:00", "18:00"); err != nil {
			t.Fatalf("insert lab_availability_general for user %d: %v", userID, err)
		}
	}

	minAge, maxAge := 6.0, 60.0
	experimentRec := do(http.MethodPost, fmt.Sprintf("/labs/%d/experiments/", labID), experimentRequest{
		Name: "Scheduling Integration Study", Status: "not_run", Sessions: 1, DurationMinutes: 30,
		AgeRangeMinMonths: &minAge, AgeRangeMaxMonths: &maxAge,
	})
	if experimentRec.Code != http.StatusCreated {
		t.Fatalf("create experiment status = %d, want %d; body = %s", experimentRec.Code, http.StatusCreated, experimentRec.Body)
	}
	var experiment experimentResponse
	decode(experimentRec, &experiment)

	if rec := do(http.MethodPost, fmt.Sprintf("/experiments/%d/training-requirements/", experiment.ID), addTrainingRequirementRequest{ExperimentRoleID: experimenterRole.ID}); rec.Code != http.StatusNoContent {
		t.Fatalf("add training requirement status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}

	var family familyResponse
	decode(do(http.MethodPost, "/families/", familyRequest{Address: "1 Main St", City: "Boulder", State: "CO", Zip: "80301"}), &family)
	var child childResponse
	decode(do(http.MethodPost, fmt.Sprintf("/families/%d/children/", family.ID), childRequest{
		FirstName: "Kid", LastName: "Test", Sex: "unknown", Response: "unknown",
	}), &child)

	// sibling_coming = "coming" makes the sitter a hard requirement.
	var appointment appointmentResponse
	appointmentRec := do(http.MethodPost, fmt.Sprintf("/experiments/%d/appointments", experiment.ID), appointmentRequest{
		ChildID: child.ID, SiblingComing: "coming",
	})
	if appointmentRec.Code != http.StatusCreated {
		t.Fatalf("create appointment status = %d, want %d; body = %s", appointmentRec.Code, http.StatusCreated, appointmentRec.Body)
	}
	decode(appointmentRec, &appointment)

	dateStr := targetDate.Format(dateLayout)
	var candidates []candidateSlotResponse
	searchRec := do(http.MethodGet, fmt.Sprintf("/appointments/%d/availability?start_date=%s&end_date=%s", appointment.ID, dateStr, dateStr), nil)
	if searchRec.Code != http.StatusOK {
		t.Fatalf("search availability status = %d, want %d; body = %s", searchRec.Code, http.StatusOK, searchRec.Body)
	}
	decode(searchRec, &candidates)
	if len(candidates) == 0 {
		t.Fatal("expected candidate slots: both candidates are free all day and the experiment needs only 30 minutes")
	}
	first := candidates[0]
	if !first.HasSitter {
		t.Errorf("HasSitter = false, want true (sibling_coming = 'coming' makes it a hard requirement)")
	}
	if first.Assignment[experimenterRole.ID] != experimenter.ID {
		t.Errorf("assignment[experimenter role %d] = %d, want %d", experimenterRole.ID, first.Assignment[experimenterRole.ID], experimenter.ID)
	}
	if first.Assignment[sitterRole.ID] != sitter.ID {
		t.Errorf("assignment[sitter role %d] = %d, want %d", sitterRole.ID, first.Assignment[sitterRole.ID], sitter.ID)
	}

	scheduleRec := do(http.MethodPost, fmt.Sprintf("/appointments/%d/schedule", appointment.ID), scheduleAppointmentRequest{
		Date: dateStr, StartTime: first.StartTime,
	})
	if scheduleRec.Code != http.StatusOK {
		t.Fatalf("schedule status = %d, want %d; body = %s", scheduleRec.Code, http.StatusOK, scheduleRec.Body)
	}
	var scheduled appointmentResponse
	decode(scheduleRec, &scheduled)
	if scheduled.Status != "pending" {
		t.Errorf("scheduled Status = %q, want %q", scheduled.Status, "pending")
	}

	// Verify the actual committed staff assignment against real SQL, not
	// just the HTTP response -- this is what a real scheduling error
	// (wrong role, wrong user, no greeter) would show up as.
	rows, err := testPool.Query(ctx,
		"select user_id, experiment_role_id, is_greeter from appointment_experimenters where appointment_id = $1 order by experiment_role_id",
		appointment.ID)
	if err != nil {
		t.Fatalf("query appointment_experimenters: %v", err)
	}
	type row struct {
		userID, roleID int64
		isGreeter      bool
	}
	var rowsGot []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.userID, &r.roleID, &r.isGreeter); err != nil {
			t.Fatalf("scan appointment_experimenters row: %v", err)
		}
		rowsGot = append(rowsGot, r)
	}
	rows.Close()
	if len(rowsGot) != 2 {
		t.Fatalf("appointment_experimenters rows = %+v, want 2", rowsGot)
	}
	greeters := 0
	for _, r := range rowsGot {
		if r.isGreeter {
			greeters++
		}
	}
	if greeters != 1 {
		t.Errorf("greeters = %d, want exactly 1", greeters)
	}

	// A second appointment with sibling_coming = "not_coming" shouldn't
	// even attempt a sitter, even though one happens to be free.
	var noSiblingAppointment appointmentResponse
	decode(do(http.MethodPost, fmt.Sprintf("/experiments/%d/appointments", experiment.ID), appointmentRequest{
		ChildID: child.ID, SiblingComing: "not_coming",
	}), &noSiblingAppointment)

	var noSiblingCandidates []candidateSlotResponse
	decode(do(http.MethodGet, fmt.Sprintf("/appointments/%d/availability?start_date=%s&end_date=%s", noSiblingAppointment.ID, dateStr, dateStr), nil), &noSiblingCandidates)
	if len(noSiblingCandidates) == 0 {
		t.Fatal("expected candidate slots for the sibling_coming=not_coming appointment")
	}
	for _, c := range noSiblingCandidates {
		if c.HasSitter {
			t.Errorf("slot %s: HasSitter = true, want false (sibling_coming = 'not_coming')", c.StartTime)
		}
		if _, ok := c.Assignment[sitterRole.ID]; ok {
			t.Errorf("slot %s: sitter role present in assignment despite sibling_coming = 'not_coming'", c.StartTime)
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
