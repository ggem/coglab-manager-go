package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

// notAMember overrides newAuthenticatedTestServer's default "always a
// member" GetLabMembership stub with one that always reports no
// membership, exercising the actual authorization decision rather than
// its bypassed-by-default fake.
func notAMember(q *dbfake.Querier) {
	q.GetLabMembershipFunc = func(ctx context.Context, arg db.GetLabMembershipParams) (db.LabMembership, error) {
		return db.LabMembership{}, pgx.ErrNoRows
	}
}

// capturingMember wires a GetLabMembershipFunc that records the arg it was
// called with and always reports membership -- for asserting the
// middleware checks the *right* lab, not just that some lab was checked
// (the default "always a member" stub used elsewhere doesn't care which
// lab_id it was passed).
func capturingMember(q *dbfake.Querier, captured *db.GetLabMembershipParams) {
	q.GetLabMembershipFunc = func(ctx context.Context, arg db.GetLabMembershipParams) (db.LabMembership, error) {
		*captured = arg
		return db.LabMembership{UserID: arg.UserID, LabID: arg.LabID}, nil
	}
}

func TestRequireLabMemberFromURL_Member(t *testing.T) {
	var captured db.GetLabMembershipParams
	q := &dbfake.Querier{
		CreateConditionFunc: func(ctx context.Context, arg db.CreateConditionParams) (db.Condition, error) {
			return db.Condition{ID: 1, LabID: arg.LabID}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	capturingMember(q, &captured)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/conditions/", cookie, conditionRequest{Name: "X"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.UserID != 7 || captured.LabID != 9 {
		t.Errorf("GetLabMembership called with %+v, want UserID=7 LabID=9 (the lab named in the URL)", captured)
	}
}

func TestRequireLabMemberForCondition_Member(t *testing.T) {
	var captured db.GetLabMembershipParams
	q := &dbfake.Querier{
		GetConditionByIDFunc: func(ctx context.Context, id int64) (db.Condition, error) {
			return db.Condition{ID: id, LabID: 9}, nil
		},
	}
	capturingMember(q, &captured)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/conditions/3/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.UserID != 7 || captured.LabID != 9 {
		t.Errorf("GetLabMembership called with %+v, want UserID=7 LabID=9 (the condition's own lab, not e.g. its ID)", captured)
	}
}

func TestRequireLabMemberForConditionValue_Member(t *testing.T) {
	var captured db.GetLabMembershipParams
	q := &dbfake.Querier{
		GetConditionValueLabIDFunc: func(ctx context.Context, id int64) (int64, error) {
			return 9, nil
		},
		DeactivateConditionValueFunc: func(ctx context.Context, id int64) error { return nil },
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	capturingMember(q, &captured)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/condition-values/5/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.UserID != 7 || captured.LabID != 9 {
		t.Errorf("GetLabMembership called with %+v, want UserID=7 LabID=9 (resolved via the parent condition)", captured)
	}
}

func TestRequireLabMemberForEquipment_Member(t *testing.T) {
	var captured db.GetLabMembershipParams
	q := &dbfake.Querier{
		GetEquipmentByIDFunc: func(ctx context.Context, id int64) (db.Equipment, error) {
			return db.Equipment{ID: id, LabID: 9}, nil
		},
	}
	capturingMember(q, &captured)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/equipment/3/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.UserID != 7 || captured.LabID != 9 {
		t.Errorf("GetLabMembership called with %+v, want UserID=7 LabID=9", captured)
	}
}

func TestRequireLabMemberForExperimentRole_Member(t *testing.T) {
	var captured db.GetLabMembershipParams
	q := &dbfake.Querier{
		GetExperimentRoleByIDFunc: func(ctx context.Context, id int64) (db.ExperimentRole, error) {
			return db.ExperimentRole{ID: id, LabID: 9}, nil
		},
	}
	capturingMember(q, &captured)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiment-roles/3/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.UserID != 7 || captured.LabID != 9 {
		t.Errorf("GetLabMembership called with %+v, want UserID=7 LabID=9", captured)
	}
}

func TestRequireLabMemberForExperiment_Member(t *testing.T) {
	var captured db.GetLabMembershipParams
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 9}, nil
		},
	}
	capturingMember(q, &captured)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/3/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.UserID != 7 || captured.LabID != 9 {
		t.Errorf("GetLabMembership called with %+v, want UserID=7 LabID=9", captured)
	}
}

func TestRequireLabMemberForExperiment_Member_JoinManagementRoute(t *testing.T) {
	var captured db.GetLabMembershipParams
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 9}, nil
		},
		AddExperimentConditionFunc: func(ctx context.Context, arg db.AddExperimentConditionParams) error { return nil },
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	capturingMember(q, &captured)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/3/conditions/", cookie, addConditionRequest{ConditionID: 5})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.UserID != 7 || captured.LabID != 9 {
		t.Errorf("GetLabMembership called with %+v, want UserID=7 LabID=9 (the experiment's lab)", captured)
	}
}

func TestRequireLabMemberFromURL_NotAMember(t *testing.T) {
	q := &dbfake.Querier{}
	notAMember(q)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/conditions/", cookie, conditionRequest{Name: "X"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRequireLabMemberForCondition_NotAMember(t *testing.T) {
	q := &dbfake.Querier{
		GetConditionByIDFunc: func(ctx context.Context, id int64) (db.Condition, error) {
			return db.Condition{ID: id, LabID: 9}, nil
		},
	}
	notAMember(q)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/conditions/3/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRequireLabMemberForConditionValue_NotAMember(t *testing.T) {
	q := &dbfake.Querier{
		GetConditionValueLabIDFunc: func(ctx context.Context, id int64) (int64, error) {
			return 9, nil
		},
	}
	notAMember(q)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/condition-values/5/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRequireLabMemberForEquipment_NotAMember(t *testing.T) {
	q := &dbfake.Querier{
		GetEquipmentByIDFunc: func(ctx context.Context, id int64) (db.Equipment, error) {
			return db.Equipment{ID: id, LabID: 9}, nil
		},
	}
	notAMember(q)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/equipment/3/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRequireLabMemberForExperimentRole_NotAMember(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentRoleByIDFunc: func(ctx context.Context, id int64) (db.ExperimentRole, error) {
			return db.ExperimentRole{ID: id, LabID: 9}, nil
		},
	}
	notAMember(q)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiment-roles/3/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRequireLabMemberForExperiment_NotAMember(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 9}, nil
		},
	}
	notAMember(q)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/3/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// The join-management routes under /experiments/{experimentID}/... are
// gated by the same requireLabMemberForExperiment middleware as the
// experiment's own routes -- one representative check here is enough to
// confirm the middleware is actually wired to that route group.
func TestRequireLabMemberForExperiment_NotAMember_JoinManagementRoute(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 9}, nil
		},
	}
	notAMember(q)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/3/conditions/", cookie, addConditionRequest{ConditionID: 5})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
