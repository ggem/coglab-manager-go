package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleCreateExperimentType_Success(t *testing.T) {
	var captured db.CreateExperimentTypeParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateExperimentTypeFunc: func(ctx context.Context, arg db.CreateExperimentTypeParams) (db.ExperimentType, error) {
			captured = arg
			return db.ExperimentType{ID: 1, LabID: arg.LabID, Name: arg.Name}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/experiment-types/", cookie, experimentTypeRequest{Name: "Longitudinal"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.LabID != 9 {
		t.Errorf("CreateExperimentType LabID = %d, want 9", captured.LabID)
	}
	got := decodeBody[experimentTypeResponse](t, rec)
	if got.ID != 1 {
		t.Errorf("response ID = %d, want 1", got.ID)
	}
	if capturedAudit.Action != ActionExperimentTypeCreated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionExperimentTypeCreated)
	}
}

func TestHandleCreateExperimentType_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodPost, "/labs/9/experiment-types/", nil, experimentTypeRequest{Name: "X"})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateExperimentType_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		CreateExperimentTypeFunc: func(ctx context.Context, arg db.CreateExperimentTypeParams) (db.ExperimentType, error) {
			return db.ExperimentType{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/experiment-types/", cookie, experimentTypeRequest{Name: "X"})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	got := decodeBody[errorResponse](t, rec)
	if got.Error != "internal error" {
		t.Errorf("error body = %q, want a generic message that doesn't leak the underlying error", got.Error)
	}
}

func TestHandleListExperimentTypesByLab_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListExperimentTypesByLabFunc: func(ctx context.Context, labID int64) ([]db.ExperimentType, error) {
			return []db.ExperimentType{{ID: 1, LabID: labID, Name: "A"}, {ID: 2, LabID: labID, Name: "B"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/experiment-types/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]experimentTypeResponse](t, rec)
	if len(got) != 2 {
		t.Errorf("len(response) = %d, want 2", len(got))
	}
}

func TestHandleUpdateExperimentType_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentTypeByIDFunc: func(ctx context.Context, id int64) (db.ExperimentType, error) {
			return db.ExperimentType{ID: id, LabID: 1}, nil
		},
		UpdateExperimentTypeFunc: func(ctx context.Context, arg db.UpdateExperimentTypeParams) (db.ExperimentType, error) {
			return db.ExperimentType{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/experiment-types/404/", cookie, experimentTypeRequest{Name: "X"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeactivateExperimentType_Success(t *testing.T) {
	var deactivatedID int64
	q := &dbfake.Querier{
		GetExperimentTypeByIDFunc: func(ctx context.Context, id int64) (db.ExperimentType, error) {
			return db.ExperimentType{ID: id, LabID: 1}, nil
		},
		DeactivateExperimentTypeFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiment-types/9/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if deactivatedID != 9 {
		t.Errorf("DeactivateExperimentType called with id = %d, want 9", deactivatedID)
	}
}
