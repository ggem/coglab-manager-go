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

func TestHandleCreateExperiment_Success(t *testing.T) {
	var captured db.CreateExperimentParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateExperimentFunc: func(ctx context.Context, arg db.CreateExperimentParams) (db.Experiment, error) {
			captured = arg
			return db.Experiment{
				ID: 1, LabID: arg.LabID, Name: arg.Name, Status: arg.Status,
				AgeRangeMinMonths: arg.AgeRangeMinMonths, StartDate: arg.StartDate,
			}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	minAge := 6.0
	startDate := "2026-01-01"
	rec := doRequest(t, s, http.MethodPost, "/labs/9/experiments/", cookie, experimentRequest{
		Name: "Looking Time Study", Status: "not_run", AgeRangeMinMonths: &minAge, StartDate: &startDate,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.LabID != 9 || captured.Name != "Looking Time Study" {
		t.Errorf("CreateExperiment params = %+v", captured)
	}
	if !captured.AgeRangeMinMonths.Valid {
		t.Errorf("CreateExperiment AgeRangeMinMonths not set from request")
	}
	if !captured.StartDate.Valid || captured.StartDate.Time.Format(dateLayout) != startDate {
		t.Errorf("CreateExperiment StartDate = %+v, want %s", captured.StartDate, startDate)
	}
	got := decodeBody[experimentResponse](t, rec)
	if got.ID != 1 || got.AgeRangeMinMonths == nil || *got.AgeRangeMinMonths != minAge {
		t.Errorf("response = %+v", got)
	}
	if got.StartDate == nil || *got.StartDate != startDate {
		t.Errorf("response StartDate = %v, want %q", got.StartDate, startDate)
	}
	if capturedAudit.Action != ActionExperimentCreated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionExperimentCreated)
	}
}

func TestHandleCreateExperiment_InvalidStartDate(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	badDate := "not-a-date"
	rec := doRequest(t, s, http.MethodPost, "/labs/9/experiments/", cookie, experimentRequest{StartDate: &badDate})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateExperiment_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodPost, "/labs/9/experiments/", nil, experimentRequest{})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateExperiment_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/not-a-number/experiments/", cookie, experimentRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateExperiment_UnknownLab(t *testing.T) {
	q := &dbfake.Querier{
		CreateExperimentFunc: func(ctx context.Context, arg db.CreateExperimentParams) (db.Experiment, error) {
			return db.Experiment{}, &pgconn.PgError{Code: pgForeignKeyViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/404/experiments/", cookie, experimentRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateExperiment_InvalidStatus(t *testing.T) {
	// status is constrained to ('not_run', 'pilot', 'run') by a check
	// constraint; the handler doesn't validate it independently.
	q := &dbfake.Querier{
		CreateExperimentFunc: func(ctx context.Context, arg db.CreateExperimentParams) (db.Experiment, error) {
			return db.Experiment{}, &pgconn.PgError{Code: pgCheckViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/experiments/", cookie, experimentRequest{Name: "X", Status: "not_a_real_status"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateExperiment_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		CreateExperimentFunc: func(ctx context.Context, arg db.CreateExperimentParams) (db.Experiment, error) {
			return db.Experiment{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/experiments/", cookie, experimentRequest{Name: "X"})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleGetExperiment_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/not-a-number/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGetExperiment_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/404/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleListExperimentsByLab_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListExperimentsByLabFunc: func(ctx context.Context, labID int64) ([]db.Experiment, error) {
			return []db.Experiment{{ID: 1, LabID: labID, Name: "A"}, {ID: 2, LabID: labID, Name: "B"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/experiments/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]experimentResponse](t, rec)
	if len(got) != 2 {
		t.Errorf("len(response) = %d, want 2", len(got))
	}
}

func TestHandleListExperimentsByLab_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/not-a-number/experiments/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateExperiment_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPut, "/experiments/not-a-number/", cookie, experimentRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateExperiment_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		UpdateExperimentFunc: func(ctx context.Context, arg db.UpdateExperimentParams) (db.Experiment, error) {
			return db.Experiment{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/experiments/404/", cookie, experimentRequest{Name: "X"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateExperiment_InvalidStatus(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		UpdateExperimentFunc: func(ctx context.Context, arg db.UpdateExperimentParams) (db.Experiment, error) {
			return db.Experiment{}, &pgconn.PgError{Code: pgCheckViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/experiments/3/", cookie, experimentRequest{Name: "X", Status: "not_a_real_status"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateExperiment_Success(t *testing.T) {
	var captured db.UpdateExperimentParams
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		UpdateExperimentFunc: func(ctx context.Context, arg db.UpdateExperimentParams) (db.Experiment, error) {
			captured = arg
			return db.Experiment{ID: arg.ID, Name: arg.Name, Status: arg.Status}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/experiments/3/", cookie, experimentRequest{Name: "Renamed Study", Status: "pilot"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.ID != 3 || captured.Name != "Renamed Study" || captured.Status != "pilot" {
		t.Errorf("UpdateExperiment params = %+v", captured)
	}
}

// TestHandleUpdateExperiment_ProtocolIDRoundTrips confirms protocol_id (a
// plain nullable FK, not a join-table endpoint like conditions/equipment)
// isn't silently dropped between the request, CreateExperiment/
// UpdateExperiment params, and the response.
func TestHandleUpdateExperiment_ProtocolIDRoundTrips(t *testing.T) {
	var captured db.UpdateExperimentParams
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		UpdateExperimentFunc: func(ctx context.Context, arg db.UpdateExperimentParams) (db.Experiment, error) {
			captured = arg
			return db.Experiment{ID: arg.ID, Name: arg.Name, Status: arg.Status, ProtocolID: arg.ProtocolID}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	protocolID := int64(5)
	rec := doRequest(t, s, http.MethodPut, "/experiments/3/", cookie, experimentRequest{
		Name: "Renamed Study", Status: "pilot", ProtocolID: &protocolID,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.ProtocolID == nil || *captured.ProtocolID != protocolID {
		t.Errorf("UpdateExperiment ProtocolID = %v, want %d", captured.ProtocolID, protocolID)
	}
	got := decodeBody[experimentResponse](t, rec)
	if got.ProtocolID == nil || *got.ProtocolID != protocolID {
		t.Errorf("response ProtocolID = %v, want %d", got.ProtocolID, protocolID)
	}
}

func TestHandleDeactivateExperiment_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/not-a-number/deactivate", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDeactivateExperiment_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		DeactivateExperimentFunc: func(ctx context.Context, id int64) error {
			return pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/404/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeactivateExperiment_Success(t *testing.T) {
	var deactivatedID int64
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		DeactivateExperimentFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if deactivatedID != 3 {
		t.Errorf("deactivated ID = %d, want 3", deactivatedID)
	}
}
