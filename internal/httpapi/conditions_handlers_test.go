package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleCreateCondition_Success(t *testing.T) {
	var captured db.CreateConditionParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateConditionFunc: func(ctx context.Context, arg db.CreateConditionParams) (db.Condition, error) {
			captured = arg
			return db.Condition{ID: 1, LabID: arg.LabID, Name: arg.Name}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/conditions/", cookie, conditionRequest{Name: "Stimulus Type"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.LabID != 9 || captured.Name != "Stimulus Type" {
		t.Errorf("CreateCondition params = %+v", captured)
	}
	got := decodeBody[conditionResponse](t, rec)
	if got.ID != 1 {
		t.Errorf("response ID = %d, want 1", got.ID)
	}
	if capturedAudit.Action != ActionConditionCreated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionConditionCreated)
	}
}

func TestHandleCreateCondition_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodPost, "/labs/9/conditions/", nil, conditionRequest{})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateCondition_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/not-a-number/conditions/", cookie, conditionRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateCondition_UnknownLab(t *testing.T) {
	// A lab_id with no matching row in labs violates the foreign key --
	// this is the "invalid lab ID" case a client would actually hit, as
	// opposed to a non-numeric ID in the URL (a different 400 above).
	q := &dbfake.Querier{
		CreateConditionFunc: func(ctx context.Context, arg db.CreateConditionParams) (db.Condition, error) {
			return db.Condition{}, &pgconn.PgError{Code: pgForeignKeyViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/404/conditions/", cookie, conditionRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateCondition_UnexpectedDBError(t *testing.T) {
	// Anything writeDBError doesn't recognize (e.g. a dropped connection)
	// must come back as a 500 without leaking the underlying error.
	q := &dbfake.Querier{
		CreateConditionFunc: func(ctx context.Context, arg db.CreateConditionParams) (db.Condition, error) {
			return db.Condition{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/conditions/", cookie, conditionRequest{Name: "X"})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	got := decodeBody[errorResponse](t, rec)
	if got.Error != "internal error" {
		t.Errorf("error body = %q, want a generic message that doesn't leak the underlying error", got.Error)
	}
}

func TestHandleListConditionsByLab_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/not-a-number/conditions/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGetCondition_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetConditionByIDFunc: func(ctx context.Context, id int64) (db.Condition, error) {
			return db.Condition{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/conditions/404/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleListConditionsByLab_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListConditionsByLabFunc: func(ctx context.Context, labID int64) ([]db.Condition, error) {
			return []db.Condition{{ID: 1, LabID: labID, Name: "A"}, {ID: 2, LabID: labID, Name: "B"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/conditions/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]conditionResponse](t, rec)
	if len(got) != 2 {
		t.Errorf("len(response) = %d, want 2", len(got))
	}
}

func TestHandleUpdateCondition_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPut, "/conditions/not-a-number/", cookie, conditionRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateCondition_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetConditionByIDFunc: func(ctx context.Context, id int64) (db.Condition, error) {
			return db.Condition{ID: id, LabID: 1}, nil
		},
		UpdateConditionFunc: func(ctx context.Context, arg db.UpdateConditionParams) (db.Condition, error) {
			return db.Condition{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/conditions/404/", cookie, conditionRequest{Name: "X"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeactivateCondition_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/conditions/not-a-number/deactivate", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDeactivateCondition_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetConditionByIDFunc: func(ctx context.Context, id int64) (db.Condition, error) {
			return db.Condition{ID: id, LabID: 1}, nil
		},
		DeactivateConditionFunc: func(ctx context.Context, id int64) error {
			return pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/conditions/404/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeactivateCondition_Success(t *testing.T) {
	var deactivatedID int64
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		GetConditionByIDFunc: func(ctx context.Context, id int64) (db.Condition, error) {
			return db.Condition{ID: id, LabID: 1}, nil
		},
		DeactivateConditionFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/conditions/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if deactivatedID != 3 {
		t.Errorf("deactivated ID = %d, want 3", deactivatedID)
	}
	if capturedAudit.Action != ActionConditionDeactivated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionConditionDeactivated)
	}
}

func TestHandleCreateConditionValue_Success(t *testing.T) {
	var captured db.CreateConditionValueParams
	q := &dbfake.Querier{
		GetConditionByIDFunc: func(ctx context.Context, id int64) (db.Condition, error) {
			return db.Condition{ID: id, LabID: 1}, nil
		},
		CreateConditionValueFunc: func(ctx context.Context, arg db.CreateConditionValueParams) (db.ConditionValue, error) {
			captured = arg
			return db.ConditionValue{ID: 1, ConditionID: arg.ConditionID, Name: arg.Name}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/conditions/3/values/", cookie, conditionValueRequest{Name: "Red"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.ConditionID != 3 || captured.Name != "Red" {
		t.Errorf("CreateConditionValue params = %+v", captured)
	}
}

func TestHandleCreateConditionValue_UnknownCondition(t *testing.T) {
	// The lab-membership middleware fetches the parent condition before the
	// handler runs, so a nonexistent condition_id now surfaces as 404 from
	// that lookup rather than an insert-time foreign key violation.
	q := &dbfake.Querier{
		GetConditionByIDFunc: func(ctx context.Context, id int64) (db.Condition, error) {
			return db.Condition{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/conditions/404/values/", cookie, conditionValueRequest{Name: "Red"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleCreateConditionValue_InvalidConditionID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/conditions/not-a-number/values/", cookie, conditionValueRequest{Name: "Red"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleListConditionValuesByCondition_Success(t *testing.T) {
	q := &dbfake.Querier{
		GetConditionByIDFunc: func(ctx context.Context, id int64) (db.Condition, error) {
			return db.Condition{ID: id, LabID: 1}, nil
		},
		ListConditionValuesByConditionFunc: func(ctx context.Context, conditionID int64) ([]db.ConditionValue, error) {
			return []db.ConditionValue{{ID: 1, ConditionID: conditionID, Name: "Red"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/conditions/3/values/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]conditionValueResponse](t, rec)
	if len(got) != 1 || got[0].Name != "Red" {
		t.Errorf("response = %+v", got)
	}
}

func TestHandleDeactivateConditionValue_Success(t *testing.T) {
	var deactivatedID int64
	q := &dbfake.Querier{
		GetConditionValueLabIDFunc: func(ctx context.Context, id int64) (int64, error) {
			return 1, nil
		},
		DeactivateConditionValueFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/condition-values/5/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if deactivatedID != 5 {
		t.Errorf("deactivated ID = %d, want 5", deactivatedID)
	}
}
