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
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/auth"
	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/dbtest"
	"github.com/ggem/coglab-manager-go/internal/mail/mailfake"
	"github.com/ggem/coglab-manager-go/internal/mcdi/mcdifake"
	"github.com/ggem/coglab-manager-go/internal/reminders"
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

	s := NewServer(auth.NewPasswordAuthenticator(testQueries), auth.NewSessionManager(testQueries, false), audit.NewRecorder(testQueries), testQueries, &mcdifake.Client{}, discardLogger())

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

	s := NewServer(auth.NewPasswordAuthenticator(testQueries), auth.NewSessionManager(testQueries, false), audit.NewRecorder(testQueries), testQueries, &mcdifake.Client{}, discardLogger())

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

	s := NewServer(auth.NewPasswordAuthenticator(testQueries), auth.NewSessionManager(testQueries, false), audit.NewRecorder(testQueries), testQueries, &mcdifake.Client{}, discardLogger())

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
	// even attempt a sitter, even though one happens to be free. Uses a
	// second child, not the first: that child's first appointment is
	// already 'pending' (a live hold, per M6's
	// appointments_one_active_hold_per_child), and this scenario isn't
	// about hold semantics.
	var secondChild childResponse
	decode(do(http.MethodPost, fmt.Sprintf("/families/%d/children/", family.ID), childRequest{
		FirstName: "SecondKid", LastName: "Test", Sex: "unknown", Response: "unknown",
	}), &secondChild)
	var noSiblingAppointment appointmentResponse
	noSiblingAppointmentRec := do(http.MethodPost, fmt.Sprintf("/experiments/%d/appointments", experiment.ID), appointmentRequest{
		ChildID: secondChild.ID, SiblingComing: "not_coming",
	})
	if noSiblingAppointmentRec.Code != http.StatusCreated {
		t.Fatalf("create second appointment status = %d, want %d; body = %s", noSiblingAppointmentRec.Code, http.StatusCreated, noSiblingAppointmentRec.Body)
	}
	decode(noSiblingAppointmentRec, &noSiblingAppointment)

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

func TestMatchingFlow_Integration(t *testing.T) {
	ctx := context.Background()

	var labID int64
	if err := testPool.QueryRow(ctx, "insert into labs (name, short_name) values ($1, $2) returning id",
		"Matching Test Lab", fmt.Sprintf("mtl-%d", time.Now().UnixNano())).Scan(&labID); err != nil {
		t.Fatalf("insert lab: %v", err)
	}

	hash, err := auth.HashPassword("s3cret-integration-test")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	actor, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email:     fmt.Sprintf("matching-actor-%d@example.edu", time.Now().UnixNano()),
		FirstName: "Actor", LastName: "Test", PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateUser(actor): %v", err)
	}

	var roleID int64
	if err := testPool.QueryRow(ctx, "insert into roles (name, description) values ($1, $2) returning id",
		fmt.Sprintf("matching-test-role-%d", time.Now().UnixNano()), "integration test role").Scan(&roleID); err != nil {
		t.Fatalf("insert role: %v", err)
	}
	if _, err := testPool.Exec(ctx, "insert into lab_memberships (user_id, lab_id, role_id) values ($1, $2, $3)", actor.ID, labID, roleID); err != nil {
		t.Fatalf("insert lab_membership: %v", err)
	}

	s := NewServer(auth.NewPasswordAuthenticator(testQueries), auth.NewSessionManager(testQueries, false), audit.NewRecorder(testQueries), testQueries, &mcdifake.Client{}, discardLogger())

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

	minAge, maxAge := 6.0, 60.0
	var experiment experimentResponse
	experimentRec := do(http.MethodPost, fmt.Sprintf("/labs/%d/experiments/", labID), experimentRequest{
		Name: "Matching Integration Study", Status: "not_run", Sessions: 1, DurationMinutes: 30,
		AgeRangeMinMonths: &minAge, AgeRangeMaxMonths: &maxAge,
		FilterPremies: true, FilterMinLanguages: 1, FilterLanguages: []string{"english"},
	})
	if experimentRec.Code != http.StatusCreated {
		t.Fatalf("create experiment status = %d, want %d; body = %s", experimentRec.Code, http.StatusCreated, experimentRec.Body)
	}
	decode(experimentRec, &experiment)

	// A second, filter-free experiment just to put "alreadyHeld" into a
	// live appointment against -- proving the hold check is global (any
	// experiment), not scoped to the one being matched against.
	var otherExperiment experimentResponse
	decode(do(http.MethodPost, fmt.Sprintf("/labs/%d/experiments/", labID), experimentRequest{
		Name: "Other Study", Status: "not_run", Sessions: 1, DurationMinutes: 30,
		AgeRangeMinMonths: ptrFloat(0), AgeRangeMaxMonths: ptrFloat(200),
	}), &otherExperiment)

	var family familyResponse
	decode(do(http.MethodPost, "/families/", familyRequest{Address: "1 Main St", City: "Boulder", State: "CO", Zip: "80301"}), &family)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	birthDateMonthsAgo := func(months int) *string {
		s := today.AddDate(0, -months, 0).Format(dateLayout)
		return &s
	}
	newChild := func(name string, req childRequest) childResponse {
		req.FirstName = name
		req.LastName = "Test"
		if req.Sex == "" {
			req.Sex = "unknown"
		}
		if req.Response == "" {
			req.Response = "unknown"
		}
		var c childResponse
		rec := do(http.MethodPost, fmt.Sprintf("/families/%d/children/", family.ID), req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create child %s status = %d, want %d; body = %s", name, rec.Code, http.StatusCreated, rec.Body)
		}
		decode(rec, &c)
		return c
	}

	eligible := newChild("Eligible", childRequest{
		BirthDate: birthDateMonthsAgo(24), Languages: []string{"english"},
	})
	wrongAge := newChild("WrongAge", childRequest{
		BirthDate: birthDateMonthsAgo(100), Languages: []string{"english"},
	})
	missingLanguage := newChild("MissingLanguage", childRequest{
		BirthDate: birthDateMonthsAgo(24), Languages: nil,
	})
	premie := newChild("Premie", childRequest{
		BirthDate: birthDateMonthsAgo(24), Languages: []string{"english"}, Premie: ptrBool(true),
	})
	alreadyHeld := newChild("AlreadyHeld", childRequest{
		BirthDate: birthDateMonthsAgo(24), Languages: []string{"english"},
	})
	if rec := do(http.MethodPost, fmt.Sprintf("/experiments/%d/appointments", otherExperiment.ID), appointmentRequest{
		ChildID: alreadyHeld.ID,
	}); rec.Code != http.StatusCreated {
		t.Fatalf("hold alreadyHeld against other experiment status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}

	windowStart := today.Format(dateLayout)
	windowEnd := today.AddDate(0, 0, 7).Format(dateLayout)

	var held []appointmentResponse
	holdRec := do(http.MethodPost, fmt.Sprintf("/experiments/%d/hold-children", experiment.ID), holdChildrenRequest{
		StartDate: windowStart, EndDate: windowEnd, Count: 10, Sort: "oldest",
	})
	if holdRec.Code != http.StatusOK {
		t.Fatalf("hold-children status = %d, want %d; body = %s", holdRec.Code, http.StatusOK, holdRec.Body)
	}
	decode(holdRec, &held)

	if len(held) != 1 || held[0].ChildID != eligible.ID {
		t.Fatalf("held = %+v, want exactly child %d (Eligible); WrongAge=%d MissingLanguage=%d Premie=%d AlreadyHeld=%d should all be excluded",
			held, eligible.ID, wrongAge.ID, missingLanguage.ID, premie.ID, alreadyHeld.ID)
	}
	heldAppointment := held[0]
	if heldAppointment.Status != "to_be_scheduled" {
		t.Errorf("held appointment Status = %q, want %q", heldAppointment.Status, "to_be_scheduled")
	}

	// A second hold-children call must find nobody new: Eligible is now
	// held (excluded by the derived hold check) and everyone else was
	// already ineligible.
	var heldAgain []appointmentResponse
	decode(do(http.MethodPost, fmt.Sprintf("/experiments/%d/hold-children", experiment.ID), holdChildrenRequest{
		StartDate: windowStart, EndDate: windowEnd, Count: 10, Sort: "oldest",
	}), &heldAgain)
	if len(heldAgain) != 0 {
		t.Fatalf("second hold-children call held = %+v, want none (Eligible is already held)", heldAgain)
	}

	// Release, then confirm the child becomes eligible again.
	releaseRec := do(http.MethodPost, fmt.Sprintf("/appointments/%d/release", heldAppointment.ID), nil)
	if releaseRec.Code != http.StatusOK {
		t.Fatalf("release status = %d, want %d; body = %s", releaseRec.Code, http.StatusOK, releaseRec.Body)
	}
	var released appointmentResponse
	decode(releaseRec, &released)
	if released.Status != "released" {
		t.Errorf("released Status = %q, want %q", released.Status, "released")
	}

	var heldAfterRelease []appointmentResponse
	decode(do(http.MethodPost, fmt.Sprintf("/experiments/%d/hold-children", experiment.ID), holdChildrenRequest{
		StartDate: windowStart, EndDate: windowEnd, Count: 10, Sort: "oldest",
	}), &heldAfterRelease)
	if len(heldAfterRelease) != 1 || heldAfterRelease[0].ChildID != eligible.ID {
		t.Fatalf("hold-children after release = %+v, want exactly child %d (Eligible) again", heldAfterRelease, eligible.ID)
	}

	// The experiment now has two appointments for Eligible (one released,
	// one freshly held) -- list, and filter by status.
	var allAppointments []appointmentResponse
	decode(do(http.MethodGet, fmt.Sprintf("/experiments/%d/appointments", experiment.ID), nil), &allAppointments)
	if len(allAppointments) != 2 {
		t.Fatalf("all appointments = %+v, want 2", allAppointments)
	}
	var releasedOnly []appointmentResponse
	decode(do(http.MethodGet, fmt.Sprintf("/experiments/%d/appointments?status=released", experiment.ID), nil), &releasedOnly)
	if len(releasedOnly) != 1 || releasedOnly[0].ID != heldAppointment.ID {
		t.Fatalf("released-only appointments = %+v, want exactly the first held appointment (%d)", releasedOnly, heldAppointment.ID)
	}
}

func ptrFloat(f float64) *float64 { return &f }
func ptrBool(b bool) *bool        { return &b }

// fastForwardToPending bypasses the M5 search/schedule flow (out of
// scope for a reporting test) by writing schedule_date/time and status
// directly, so handleArriveAppointment has a 'pending' row to transition
// -- the arrive endpoint itself is exercised for real; only getting to
// "scheduled" is short-circuited.
func fastForwardToPending(ctx context.Context, t *testing.T, appointmentID int64, date time.Time) {
	t.Helper()
	if _, err := testPool.Exec(ctx,
		"update appointments set schedule_date = $1, schedule_time_start = '09:00', schedule_time_end = '09:30', status = 'pending' where id = $2",
		date, appointmentID); err != nil {
		t.Fatalf("fast-forward appointment %d to pending: %v", appointmentID, err)
	}
}

func TestReportingFlow_Integration(t *testing.T) {
	ctx := context.Background()

	var labID int64
	if err := testPool.QueryRow(ctx, "insert into labs (name, short_name) values ($1, $2) returning id",
		"Reporting Test Lab", fmt.Sprintf("rtl-%d", time.Now().UnixNano())).Scan(&labID); err != nil {
		t.Fatalf("insert lab: %v", err)
	}

	hash, err := auth.HashPassword("s3cret-integration-test")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	actor, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email:     fmt.Sprintf("reporting-actor-%d@example.edu", time.Now().UnixNano()),
		FirstName: "Actor", LastName: "Test", PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateUser(actor): %v", err)
	}

	var roleID int64
	if err := testPool.QueryRow(ctx, "insert into roles (name, description) values ($1, $2) returning id",
		fmt.Sprintf("reporting-test-role-%d", time.Now().UnixNano()), "integration test role").Scan(&roleID); err != nil {
		t.Fatalf("insert role: %v", err)
	}
	if _, err := testPool.Exec(ctx, "insert into lab_memberships (user_id, lab_id, role_id) values ($1, $2, $3)", actor.ID, labID, roleID); err != nil {
		t.Fatalf("insert lab_membership: %v", err)
	}

	s := NewServer(auth.NewPasswordAuthenticator(testQueries), auth.NewSessionManager(testQueries, false), audit.NewRecorder(testQueries), testQueries, &mcdifake.Client{}, discardLogger())

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

	// A zip unique to this test run: ZipCodesReport is deliberately not
	// lab-scoped for its child counts (children aren't lab-scoped data),
	// so a fixed zip shared with another integration test's families
	// (e.g. TestMatchingFlow_Integration also uses Boulder addresses)
	// would inflate this test's count with unrelated children.
	testZip := fmt.Sprintf("%05d", time.Now().UnixNano()%100000)

	var protocol protocolResponse
	decode(do(http.MethodPost, fmt.Sprintf("/labs/%d/protocols/", labID), protocolRequest{Name: "IRB-2026-001"}), &protocol)
	var grant grantResponse
	decode(do(http.MethodPost, fmt.Sprintf("/labs/%d/grants/", labID), grantRequest{Name: "NIH R01 Test Grant"}), &grant)
	var zipCode zipCodeResponse
	decode(do(http.MethodPost, fmt.Sprintf("/labs/%d/zip-codes/", labID), zipCodeRequest{ZipCode: testZip, Priority: "high"}), &zipCode)

	minAge, maxAge := 0.0, 240.0
	var experiment experimentResponse
	experimentRec := do(http.MethodPost, fmt.Sprintf("/labs/%d/experiments/", labID), experimentRequest{
		Name: "Reporting Integration Study", Status: "not_run", Sessions: 1, DurationMinutes: 30,
		AgeRangeMinMonths: &minAge, AgeRangeMaxMonths: &maxAge, ProtocolID: &protocol.ID,
	})
	if experimentRec.Code != http.StatusCreated {
		t.Fatalf("create experiment status = %d, want %d; body = %s", experimentRec.Code, http.StatusCreated, experimentRec.Body)
	}
	decode(experimentRec, &experiment)
	if rec := do(http.MethodPost, fmt.Sprintf("/experiments/%d/grants/", experiment.ID), addGrantRequest{GrantID: grant.ID}); rec.Code != http.StatusNoContent {
		t.Fatalf("add experiment grant status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}

	var family familyResponse
	decode(do(http.MethodPost, "/families/", familyRequest{Address: "1 Main St", City: "Boulder", State: "CO", Zip: testZip}), &family)

	// Child A: white, female. Child B: white + asian, male -- picked to
	// prove NIH's per-category counting (both count toward "white") vs.
	// its distinct-child totals (male=1, female=1, not summed from the
	// category rows).
	var childA, childB childResponse
	decode(do(http.MethodPost, fmt.Sprintf("/families/%d/children/", family.ID), childRequest{
		FirstName: "ChildA", LastName: "Test", Sex: "female", Response: "unknown",
		BirthDate: ptrDateYearsAgo(2), RaceEthnicity: []string{"white"},
	}), &childA)
	decode(do(http.MethodPost, fmt.Sprintf("/families/%d/children/", family.ID), childRequest{
		FirstName: "ChildB", LastName: "Test", Sex: "male", Response: "unknown",
		BirthDate: ptrDateYearsAgo(3), RaceEthnicity: []string{"white", "asian"},
	}), &childB)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	windowStart, windowEnd := today.AddDate(0, 0, -1).Format(dateLayout), today.AddDate(0, 0, 1).Format(dateLayout)

	var apptA, apptB appointmentResponse
	decode(do(http.MethodPost, fmt.Sprintf("/experiments/%d/appointments", experiment.ID), appointmentRequest{ChildID: childA.ID}), &apptA)
	decode(do(http.MethodPost, fmt.Sprintf("/experiments/%d/appointments", experiment.ID), appointmentRequest{ChildID: childB.ID}), &apptB)
	fastForwardToPending(ctx, t, apptA.ID, today)
	fastForwardToPending(ctx, t, apptB.ID, today)
	for _, id := range []int64{apptA.ID, apptB.ID} {
		if rec := do(http.MethodPost, fmt.Sprintf("/appointments/%d/arrive", id), nil); rec.Code != http.StatusOK {
			t.Fatalf("arrive appointment %d status = %d, want %d; body = %s", id, rec.Code, http.StatusOK, rec.Body)
		}
	}

	// NIH: white has both children (1 male, 1 female); asian has only
	// ChildB (1 male). Totals must be distinct-child (1 male, 1 female),
	// not a sum across categories.
	var nih nihReportResponse
	decode(do(http.MethodGet, fmt.Sprintf("/labs/%d/reports/nih?start_date=%s&end_date=%s&grant_id=%d", labID, windowStart, windowEnd, grant.ID), nil), &nih)
	byCategory := map[string]nihReportCategoryRow{}
	for _, c := range nih.Categories {
		byCategory[c.Category] = c
	}
	if white := byCategory["white"]; white.Male != 1 || white.Female != 1 {
		t.Errorf("NIH white row = %+v, want male=1 female=1", white)
	}
	if asian := byCategory["asian"]; asian.Male != 1 || asian.Female != 0 {
		t.Errorf("NIH asian row = %+v, want male=1 female=0", asian)
	}
	if nih.Totals.Male != 1 || nih.Totals.Female != 1 {
		t.Errorf("NIH totals = %+v, want male=1 female=1 (distinct children, not summed)", nih.Totals)
	}

	// HRC: both appointments are under our one protocol.
	var hrc hrcReportResponse
	decode(do(http.MethodGet, fmt.Sprintf("/labs/%d/reports/hrc?start_date=%s&end_date=%s", labID, windowStart, windowEnd), nil), &hrc)
	if hrc.Total != 2 {
		t.Errorf("HRC total = %d, want 2", hrc.Total)
	}
	var ourProtocolRow *hrcReportProtocolRow
	for i, p := range hrc.Protocols {
		if p.ProtocolID != nil && *p.ProtocolID == protocol.ID {
			ourProtocolRow = &hrc.Protocols[i]
		}
	}
	if ourProtocolRow == nil || ourProtocolRow.ChildCount != 2 {
		t.Errorf("HRC row for protocol %d = %+v, want child_count=2", protocol.ID, ourProtocolRow)
	}

	// Demographics: per-experiment, both children arrived in window.
	var demographics demographicsReportResponse
	decode(do(http.MethodGet, fmt.Sprintf("/experiments/%d/reports/demographics?start_date=%s&end_date=%s", experiment.ID, windowStart, windowEnd), nil), &demographics)
	if demographics.Summary.Count != 2 {
		t.Fatalf("demographics summary count = %d, want 2; full = %+v", demographics.Summary.Count, demographics)
	}
	if demographics.Summary.BySex["male"] != 1 || demographics.Summary.BySex["female"] != 1 {
		t.Errorf("demographics by_sex = %+v, want male=1 female=1", demographics.Summary.BySex)
	}
	if demographics.Summary.ByRaceEthnicity["white"] != 2 || demographics.Summary.ByRaceEthnicity["asian"] != 1 {
		t.Errorf("demographics by_race_ethnicity = %+v, want white=2 asian=1", demographics.Summary.ByRaceEthnicity)
	}
	if demographics.Summary.AgeMonthsMin <= 0 || demographics.Summary.AgeMonthsMax <= demographics.Summary.AgeMonthsMin {
		t.Errorf("demographics age summary = min %v max %v, want a real spread (children are 2 and 3 years old)",
			demographics.Summary.AgeMonthsMin, demographics.Summary.AgeMonthsMax)
	}

	// Zip codes: both children share family 1's zip, which has a
	// configured priority.
	var zipReport []zipCodesReportRow
	decode(do(http.MethodGet, fmt.Sprintf("/labs/%d/reports/zip-codes", labID), nil), &zipReport)
	var ourZipRow *zipCodesReportRow
	for i, z := range zipReport {
		if z.Zip == testZip {
			ourZipRow = &zipReport[i]
		}
	}
	if ourZipRow == nil || ourZipRow.ChildCount != 2 || ourZipRow.Priority == nil || *ourZipRow.Priority != "high" {
		t.Fatalf("zip codes row for %s = %+v, want child_count=2 priority=high", testZip, ourZipRow)
	}
}

func ptrDateYearsAgo(years int) *string {
	s := time.Now().UTC().AddDate(-years, 0, 0).Format(dateLayout)
	return &s
}

func TestNewsletterExportFlow_Integration(t *testing.T) {
	ctx := context.Background()

	var labID int64
	if err := testPool.QueryRow(ctx, "insert into labs (name, short_name) values ($1, $2) returning id",
		"Newsletter Test Lab", fmt.Sprintf("ntl-%d", time.Now().UnixNano())).Scan(&labID); err != nil {
		t.Fatalf("insert lab: %v", err)
	}

	hash, err := auth.HashPassword("s3cret-integration-test")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	actor, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email:     fmt.Sprintf("newsletter-actor-%d@example.edu", time.Now().UnixNano()),
		FirstName: "Actor", LastName: "Test", PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateUser(actor): %v", err)
	}

	var roleID int64
	if err := testPool.QueryRow(ctx, "insert into roles (name, description) values ($1, $2) returning id",
		fmt.Sprintf("newsletter-test-role-%d", time.Now().UnixNano()), "integration test role").Scan(&roleID); err != nil {
		t.Fatalf("insert role: %v", err)
	}
	if _, err := testPool.Exec(ctx, "insert into lab_memberships (user_id, lab_id, role_id) values ($1, $2, $3)", actor.ID, labID, roleID); err != nil {
		t.Fatalf("insert lab_membership: %v", err)
	}

	s := NewServer(auth.NewPasswordAuthenticator(testQueries), auth.NewSessionManager(testQueries, false), audit.NewRecorder(testQueries), testQueries, &mcdifake.Client{}, discardLogger())

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

	minAge, maxAge := 0.0, 240.0
	var experiment experimentResponse
	decode(do(http.MethodPost, fmt.Sprintf("/labs/%d/experiments/", labID), experimentRequest{
		Name: "Newsletter Integration Study", Status: "not_run", Sessions: 1, DurationMinutes: 30,
		AgeRangeMinMonths: &minAge, AgeRangeMaxMonths: &maxAge,
	}), &experiment)

	var family familyResponse
	decode(do(http.MethodPost, "/families/", familyRequest{Address: "42 Elm St", City: "Boulder", State: "CO", Zip: "80302"}), &family)
	var guardian guardianResponse
	decode(do(http.MethodPost, fmt.Sprintf("/families/%d/guardians/", family.ID), guardianRequest{
		FirstName: "Parent", LastName: "One", Education: "unknown",
	}), &guardian)
	var child childResponse
	decode(do(http.MethodPost, fmt.Sprintf("/families/%d/children/", family.ID), childRequest{
		FirstName: "Kid", LastName: "Test", Sex: "unknown", Response: "unknown",
	}), &child)

	var appointment appointmentResponse
	decode(do(http.MethodPost, fmt.Sprintf("/experiments/%d/appointments", experiment.ID), appointmentRequest{ChildID: child.ID}), &appointment)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	fastForwardToPending(ctx, t, appointment.ID, today)
	if rec := do(http.MethodPost, fmt.Sprintf("/appointments/%d/arrive", appointment.ID), nil); rec.Code != http.StatusOK {
		t.Fatalf("arrive appointment status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}

	var newsletter newsletterResponse
	decode(do(http.MethodPost, fmt.Sprintf("/labs/%d/newsletters/", labID), newsletterRequest{Name: "Fall Update"}), &newsletter)

	windowStart, windowEnd := today.AddDate(0, 0, -1).Format(dateLayout), today.AddDate(0, 0, 1).Format(dateLayout)

	// Export without a newsletter filter: the family is eligible.
	exportRec := do(http.MethodGet, fmt.Sprintf("/labs/%d/newsletters/export?start_date=%s&end_date=%s", labID, windowStart, windowEnd), nil)
	if exportRec.Code != http.StatusOK {
		t.Fatalf("export status = %d, want %d; body = %s", exportRec.Code, http.StatusOK, exportRec.Body)
	}
	if ct := exportRec.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/csv")
	}
	rows, err := csv.NewReader(exportRec.Body).ReadAll()
	if err != nil {
		t.Fatalf("parse export CSV: %v", err)
	}
	if len(rows) != 1 || rows[0][0] != "Parent" || rows[0][1] != "One" || rows[0][2] != "42 Elm St" || rows[0][5] != "80302" {
		t.Fatalf("export rows = %+v, want one row for Parent One at 42 Elm St / 80302", rows)
	}

	// Mark sent for this newsletter, then confirm a second export
	// (filtered to the same newsletter) excludes the now-marked family.
	markRec := do(http.MethodPost, fmt.Sprintf("/newsletters/%d/mark-sent?start_date=%s&end_date=%s", newsletter.ID, windowStart, windowEnd), nil)
	if markRec.Code != http.StatusOK {
		t.Fatalf("mark-sent status = %d, want %d; body = %s", markRec.Code, http.StatusOK, markRec.Body)
	}
	var markResult map[string]int
	decode(markRec, &markResult)
	if markResult["marked_sent"] != 1 {
		t.Errorf("marked_sent = %d, want 1", markResult["marked_sent"])
	}

	exportAfterMarkRec := do(http.MethodGet, fmt.Sprintf("/labs/%d/newsletters/export?start_date=%s&end_date=%s&newsletter_id=%d", labID, windowStart, windowEnd, newsletter.ID), nil)
	rowsAfterMark, err := csv.NewReader(exportAfterMarkRec.Body).ReadAll()
	if err != nil {
		t.Fatalf("parse post-mark export CSV: %v", err)
	}
	if len(rowsAfterMark) != 0 {
		t.Fatalf("post-mark-sent export (filtered to this newsletter) = %+v, want no rows", rowsAfterMark)
	}

	// An export with no newsletter filter still sees the family --
	// mark-sent only excludes future exports scoped to that newsletter.
	exportNoFilterRec := do(http.MethodGet, fmt.Sprintf("/labs/%d/newsletters/export?start_date=%s&end_date=%s", labID, windowStart, windowEnd), nil)
	rowsNoFilter, err := csv.NewReader(exportNoFilterRec.Body).ReadAll()
	if err != nil {
		t.Fatalf("parse unfiltered export CSV: %v", err)
	}
	if len(rowsNoFilter) != 1 {
		t.Fatalf("unfiltered export after mark-sent = %+v, want still 1 row (no newsletter_id filter applied)", rowsNoFilter)
	}
}

// TestStaffDigestFlow_Integration exercises internal/reminders.
// RunStaffDigest against real Postgres. It builds appointment/staff
// state directly through db.Queries (the same handle RunStaffDigest
// itself takes) rather than through the HTTP layer or a full M5
// search+schedule flow -- that flow already has its own integration
// coverage (TestSchedulingFlow_Integration); this test's job is proving
// the digest's own audit_events-driven query logic against a real
// database, so it fabricates a scheduled appointment and its
// appointment.scheduled audit event directly.
func TestStaffDigestFlow_Integration(t *testing.T) {
	ctx := context.Background()

	// Prime the digest's cursor first: a job with no stored cursor yet
	// deliberately initializes one and skips notifying on that pass
	// (see RunStaffDigest's doc comment) rather than catching up on
	// every historical change -- so this test's own scheduling change
	// below needs the cursor to already exist, the same way it would on
	// any run after the very first one in a real deployment.
	if err := reminders.RunStaffDigest(ctx, testQueries, &mailfake.Sender{}, discardLogger(), time.Now().UTC()); err != nil {
		t.Fatalf("RunStaffDigest (priming call): %v", err)
	}

	var labID int64
	if err := testPool.QueryRow(ctx, "insert into labs (name, short_name) values ($1, $2) returning id",
		"Digest Test Lab", fmt.Sprintf("dtl-%d", time.Now().UnixNano())).Scan(&labID); err != nil {
		t.Fatalf("insert lab: %v", err)
	}

	experimenter, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email: fmt.Sprintf("digest-staff-%d@example.edu", time.Now().UnixNano()), FirstName: "Staff", LastName: "Member",
	})
	if err != nil {
		t.Fatalf("CreateUser(experimenter): %v", err)
	}

	role, err := testQueries.CreateExperimentRole(ctx, db.CreateExperimentRoleParams{LabID: labID, Name: "Experimenter"})
	if err != nil {
		t.Fatalf("CreateExperimentRole: %v", err)
	}

	minAge, maxAge := numericFor(t, 0), numericFor(t, 240)
	experiment, err := testQueries.CreateExperiment(ctx, db.CreateExperimentParams{
		LabID: labID, Name: "Digest Integration Study", Status: "not_run", Sessions: 1,
		AgeRangeMinMonths: minAge, AgeRangeMaxMonths: maxAge, DurationMinutes: 30,
		FilterLanguages: []string{},
	})
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}

	family, err := testQueries.CreateFamily(ctx, db.CreateFamilyParams{Address: "1 Main St", City: "Boulder", State: "CO", Zip: "80301"})
	if err != nil {
		t.Fatalf("CreateFamily: %v", err)
	}
	child, err := testQueries.CreateChild(ctx, db.CreateChildParams{
		FamilyID: family.ID, FirstName: "Kid", LastName: "Test", Sex: "unknown",
		RaceEthnicity: []string{}, Languages: []string{}, Response: "unknown", CreatedByUserID: experimenter.ID,
	})
	if err != nil {
		t.Fatalf("CreateChild: %v", err)
	}

	appointment, err := testQueries.CreateAppointment(ctx, db.CreateAppointmentParams{
		ExperimentID: experiment.ID, ChildID: child.ID, Session: 1, SiblingComing: "unknown",
	})
	if err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}

	tomorrow := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 1)
	scheduled, err := testQueries.ScheduleAppointment(ctx, db.ScheduleAppointmentParams{
		ID: appointment.ID, ScheduleDate: pgtype.Date{Time: tomorrow, Valid: true},
		ScheduleTimeStart: pgtype.Time{Microseconds: int64(9 * time.Hour / time.Microsecond), Valid: true},
		ScheduleTimeEnd:   pgtype.Time{Microseconds: int64(9*time.Hour/time.Microsecond) + int64(30*time.Minute/time.Microsecond), Valid: true},
	})
	if err != nil {
		t.Fatalf("ScheduleAppointment: %v", err)
	}
	if _, err := testQueries.CreateAppointmentExperimenter(ctx, db.CreateAppointmentExperimenterParams{
		AppointmentID: scheduled.ID, UserID: experimenter.ID, ExperimentRoleID: role.ID,
	}); err != nil {
		t.Fatalf("CreateAppointmentExperimenter: %v", err)
	}
	if _, err := testPool.Exec(ctx,
		"insert into audit_events (actor_user_id, lab_id, action, entity_type, entity_id) values ($1, $2, 'appointment.scheduled', 'appointment', $3)",
		experimenter.ID, labID, scheduled.ID); err != nil {
		t.Fatalf("insert audit event: %v", err)
	}

	sender := &mailfake.Sender{}
	now := time.Now().UTC()
	if err := reminders.RunStaffDigest(ctx, testQueries, sender, discardLogger(), now); err != nil {
		t.Fatalf("RunStaffDigest: %v", err)
	}

	got := sender.Messages()
	if len(got) != 1 {
		t.Fatalf("Messages = %+v, want exactly one", got)
	}
	if got[0].To != experimenter.Email {
		t.Errorf("To = %q, want %q", got[0].To, experimenter.Email)
	}
	if !strings.Contains(got[0].Body, "Kid") || !strings.Contains(got[0].Body, "Digest Integration Study") {
		t.Errorf("Body = %q, want it to mention the child and experiment", got[0].Body)
	}

	// A second run with a later now, nothing new scheduled, must not
	// re-send: the cursor advanced past this appointment's audit event.
	if err := reminders.RunStaffDigest(ctx, testQueries, sender, discardLogger(), now.Add(time.Minute)); err != nil {
		t.Fatalf("RunStaffDigest (second run): %v", err)
	}
	if len(sender.Messages()) != 1 {
		t.Fatalf("Messages after second run = %+v, want still exactly one (no duplicate)", sender.Messages())
	}
}

// TestFamilyReminderFlow_Integration exercises internal/reminders.
// RunFamilyReminders against real Postgres, for the same reason
// TestStaffDigestFlow_Integration builds its appointment state directly
// through db.Queries rather than a full HTTP/scheduling flow.
func TestFamilyReminderFlow_Integration(t *testing.T) {
	ctx := context.Background()

	family, err := testQueries.CreateFamily(ctx, db.CreateFamilyParams{Address: "42 Elm St", City: "Boulder", State: "CO", Zip: "80302"})
	if err != nil {
		t.Fatalf("CreateFamily: %v", err)
	}
	guardianEmail := fmt.Sprintf("guardian-%d@example.edu", time.Now().UnixNano())
	if _, err := testQueries.CreateGuardian(ctx, db.CreateGuardianParams{
		FamilyID: family.ID, FirstName: "Parent", LastName: "One", Education: "unknown", Email: guardianEmail,
	}); err != nil {
		t.Fatalf("CreateGuardian: %v", err)
	}

	actor, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email: fmt.Sprintf("reminder-actor-%d@example.edu", time.Now().UnixNano()), FirstName: "Actor", LastName: "Test",
	})
	if err != nil {
		t.Fatalf("CreateUser(actor): %v", err)
	}
	child, err := testQueries.CreateChild(ctx, db.CreateChildParams{
		FamilyID: family.ID, FirstName: "Kid", LastName: "Test", Sex: "unknown",
		RaceEthnicity: []string{}, Languages: []string{}, Response: "unknown", CreatedByUserID: actor.ID,
	})
	if err != nil {
		t.Fatalf("CreateChild: %v", err)
	}

	var labID int64
	if err := testPool.QueryRow(ctx, "insert into labs (name, short_name) values ($1, $2) returning id",
		"Reminder Test Lab", fmt.Sprintf("rtl2-%d", time.Now().UnixNano())).Scan(&labID); err != nil {
		t.Fatalf("insert lab: %v", err)
	}
	minAge, maxAge := numericFor(t, 0), numericFor(t, 240)
	experiment, err := testQueries.CreateExperiment(ctx, db.CreateExperimentParams{
		LabID: labID, Name: "Reminder Integration Study", Status: "not_run", Sessions: 1,
		AgeRangeMinMonths: minAge, AgeRangeMaxMonths: maxAge, DurationMinutes: 30,
		FilterLanguages: []string{},
	})
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}

	appointment, err := testQueries.CreateAppointment(ctx, db.CreateAppointmentParams{
		ExperimentID: experiment.ID, ChildID: child.ID, Session: 1, SiblingComing: "unknown",
	})
	if err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}
	now := time.Now().UTC()
	// 12 hours out: inside a 24h reminder lead time.
	soon := now.Add(12 * time.Hour)
	if _, err := testQueries.ScheduleAppointment(ctx, db.ScheduleAppointmentParams{
		ID: appointment.ID, ScheduleDate: pgtype.Date{Time: time.Date(soon.Year(), soon.Month(), soon.Day(), 0, 0, 0, 0, time.UTC), Valid: true},
		ScheduleTimeStart: pgtype.Time{Microseconds: int64(soon.Hour())*int64(time.Hour/time.Microsecond) + int64(soon.Minute())*int64(time.Minute/time.Microsecond), Valid: true},
		ScheduleTimeEnd:   pgtype.Time{Microseconds: int64(soon.Hour())*int64(time.Hour/time.Microsecond) + int64(soon.Minute())*int64(time.Minute/time.Microsecond) + int64(30*time.Minute/time.Microsecond), Valid: true},
	}); err != nil {
		t.Fatalf("ScheduleAppointment: %v", err)
	}

	sender := &mailfake.Sender{}
	if err := reminders.RunFamilyReminders(ctx, testQueries, sender, discardLogger(), now, 24*time.Hour); err != nil {
		t.Fatalf("RunFamilyReminders: %v", err)
	}

	got := sender.Messages()
	if len(got) != 1 {
		t.Fatalf("Messages = %+v, want exactly one", got)
	}
	if got[0].To != guardianEmail {
		t.Errorf("To = %q, want %q", got[0].To, guardianEmail)
	}
	if !strings.Contains(got[0].Body, "Kid") || !strings.Contains(got[0].Body, "Reminder Integration Study") {
		t.Errorf("Body = %q, want it to mention the child and experiment", got[0].Body)
	}

	var reminderSentAt pgtype.Timestamptz
	if err := testPool.QueryRow(ctx, "select reminder_sent_at from appointments where id = $1", appointment.ID).Scan(&reminderSentAt); err != nil {
		t.Fatalf("query reminder_sent_at: %v", err)
	}
	if !reminderSentAt.Valid {
		t.Error("reminder_sent_at was not stamped")
	}

	// A second run must not re-send: reminder_sent_at excludes it now.
	if err := reminders.RunFamilyReminders(ctx, testQueries, sender, discardLogger(), now, 24*time.Hour); err != nil {
		t.Fatalf("RunFamilyReminders (second run): %v", err)
	}
	if len(sender.Messages()) != 1 {
		t.Fatalf("Messages after second run = %+v, want still exactly one (no duplicate)", sender.Messages())
	}
}

func numericFor(t *testing.T, f float64) pgtype.Numeric {
	t.Helper()
	n, err := ptrToNumeric(&f)
	if err != nil {
		t.Fatalf("ptrToNumeric(%v): %v", f, err)
	}
	return n
}

// TestRequestMCDIFlow_Integration exercises handleRequestMCDI end to
// end against real Postgres, with a fake mcdi.Client standing in for
// daxlabbase/cdibase (the same technique M8's tests use mailfake.Sender
// for) so the test can assert on exactly what was requested without a
// real external service.
func TestRequestMCDIFlow_Integration(t *testing.T) {
	ctx := context.Background()

	hash, err := auth.HashPassword("s3cret-integration-test")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	actor, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email: fmt.Sprintf("mcdi-actor-%d@example.edu", time.Now().UnixNano()), FirstName: "Actor", LastName: "Test", PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateUser(actor): %v", err)
	}

	family, err := testQueries.CreateFamily(ctx, db.CreateFamilyParams{Address: "1 Main St", City: "Boulder", State: "CO", Zip: "80301"})
	if err != nil {
		t.Fatalf("CreateFamily: %v", err)
	}
	guardianEmail := fmt.Sprintf("guardian-%d@example.edu", time.Now().UnixNano())
	if _, err := testQueries.CreateGuardian(ctx, db.CreateGuardianParams{
		FamilyID: family.ID, FirstName: "Parent", LastName: "One", Education: "unknown", Email: guardianEmail,
	}); err != nil {
		t.Fatalf("CreateGuardian: %v", err)
	}
	child, err := testQueries.CreateChild(ctx, db.CreateChildParams{
		FamilyID: family.ID, FirstName: "Kid", LastName: "Test", Sex: "male",
		BirthDate:     pgtype.Date{Time: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), Valid: true},
		RaceEthnicity: []string{}, Languages: []string{}, Response: "unknown", CreatedByUserID: actor.ID,
	})
	if err != nil {
		t.Fatalf("CreateChild: %v", err)
	}

	mcdiClient := &mcdifake.Client{}
	s := NewServer(auth.NewPasswordAuthenticator(testQueries), auth.NewSessionManager(testQueries, false), audit.NewRecorder(testQueries), testQueries, mcdiClient, discardLogger())

	loginRec := postJSON(t, s, "/login", loginRequest{Email: actor.Email, Password: "s3cret-integration-test"})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d; body = %s", loginRec.Code, http.StatusOK, loginRec.Body)
	}
	cookie := loginRec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/children/%d/request-mcdi", child.ID), nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("request-mcdi status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}

	sent := mcdiClient.Sent()
	if len(sent) != 1 {
		t.Fatalf("Sent() = %+v, want exactly one request", sent)
	}
	if sent[0].ChildName != "Kid Test" || sent[0].ParentEmail != guardianEmail || sent[0].Gender != "male" || sent[0].Birthday != "2024-01-15" || sent[0].DatabaseID != child.ID {
		t.Errorf("request = %+v, want ChildName=Kid Test ParentEmail=%s Gender=male Birthday=2024-01-15 DatabaseID=%d", sent[0], guardianEmail, child.ID)
	}

	actions, err := auditEventsForUser(ctx, actor.ID)
	if err != nil {
		t.Fatalf("auditEventsForUser: %v", err)
	}
	found := false
	for _, a := range actions {
		if a == ActionMCDIRequested {
			found = true
		}
	}
	if !found {
		t.Errorf("audit actions = %v, want %q among them", actions, ActionMCDIRequested)
	}
}

// TestRequestMCDIFlow_NoGuardianEmail_Integration confirms the 400 path
// against a real family with a guardian on file but no email address.
func TestRequestMCDIFlow_NoGuardianEmail_Integration(t *testing.T) {
	ctx := context.Background()

	hash, err := auth.HashPassword("s3cret-integration-test")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	actor, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email: fmt.Sprintf("mcdi-noemail-actor-%d@example.edu", time.Now().UnixNano()), FirstName: "Actor", LastName: "Test", PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateUser(actor): %v", err)
	}

	family, err := testQueries.CreateFamily(ctx, db.CreateFamilyParams{Address: "1 Main St", City: "Boulder", State: "CO", Zip: "80301"})
	if err != nil {
		t.Fatalf("CreateFamily: %v", err)
	}
	if _, err := testQueries.CreateGuardian(ctx, db.CreateGuardianParams{
		FamilyID: family.ID, FirstName: "Parent", LastName: "One", Education: "unknown", Email: "",
	}); err != nil {
		t.Fatalf("CreateGuardian: %v", err)
	}
	child, err := testQueries.CreateChild(ctx, db.CreateChildParams{
		FamilyID: family.ID, FirstName: "Kid", LastName: "Test", Sex: "male",
		BirthDate:     pgtype.Date{Time: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), Valid: true},
		RaceEthnicity: []string{}, Languages: []string{}, Response: "unknown", CreatedByUserID: actor.ID,
	})
	if err != nil {
		t.Fatalf("CreateChild: %v", err)
	}

	mcdiClient := &mcdifake.Client{}
	s := NewServer(auth.NewPasswordAuthenticator(testQueries), auth.NewSessionManager(testQueries, false), audit.NewRecorder(testQueries), testQueries, mcdiClient, discardLogger())

	loginRec := postJSON(t, s, "/login", loginRequest{Email: actor.Email, Password: "s3cret-integration-test"})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d; body = %s", loginRec.Code, http.StatusOK, loginRec.Body)
	}
	cookie := loginRec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/children/%d/request-mcdi", child.ID), nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("request-mcdi status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body)
	}
	if len(mcdiClient.Sent()) != 0 {
		t.Errorf("Sent() = %+v, want none", mcdiClient.Sent())
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
