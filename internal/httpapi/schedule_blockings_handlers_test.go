package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleCreateScheduleBlocking_Success(t *testing.T) {
	var captured db.CreateScheduleBlockingParams
	q := &dbfake.Querier{
		CreateScheduleBlockingFunc: func(ctx context.Context, arg db.CreateScheduleBlockingParams) (db.ScheduleBlocking, error) {
			captured = arg
			return db.ScheduleBlocking{ID: 1, LabID: arg.LabID, Date: arg.Date, StartTime: arg.StartTime, EndTime: arg.EndTime, Reason: arg.Reason}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/schedule-blockings/", cookie, scheduleBlockingRequest{
		Date: "2026-12-25", StartTime: "00:00", EndTime: "23:55", Reason: "Holiday",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.LabID != 9 || captured.Reason != "Holiday" {
		t.Errorf("CreateScheduleBlocking params = %+v", captured)
	}
}

func TestHandleCreateScheduleBlocking_InvalidTime(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/schedule-blockings/", cookie, scheduleBlockingRequest{
		Date: "2026-12-25", StartTime: "nope", EndTime: "23:55",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateScheduleBlocking_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		CreateScheduleBlockingFunc: func(ctx context.Context, arg db.CreateScheduleBlockingParams) (db.ScheduleBlocking, error) {
			return db.ScheduleBlocking{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/schedule-blockings/", cookie, scheduleBlockingRequest{
		Date: "2026-12-25", StartTime: "00:00", EndTime: "23:55", Reason: "Holiday",
	})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleListScheduleBlockingsByLab_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		ListScheduleBlockingsByLabFunc: func(ctx context.Context, labID int64) ([]db.ScheduleBlocking, error) {
			return nil, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/schedule-blockings/", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleListScheduleBlockingsByLab_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListScheduleBlockingsByLabFunc: func(ctx context.Context, labID int64) ([]db.ScheduleBlocking, error) {
			return []db.ScheduleBlocking{{ID: 1, LabID: labID}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/schedule-blockings/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
}

func TestHandleDeactivateScheduleBlocking_Success(t *testing.T) {
	var deactivatedID int64
	q := &dbfake.Querier{
		GetScheduleBlockingByIDFunc: func(ctx context.Context, id int64) (db.ScheduleBlocking, error) {
			return db.ScheduleBlocking{ID: id, LabID: 1}, nil
		},
		DeactivateScheduleBlockingFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/schedule-blockings/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if deactivatedID != 3 {
		t.Errorf("deactivated ID = %d, want 3", deactivatedID)
	}
}

func TestHandleDeactivateScheduleBlocking_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetScheduleBlockingByIDFunc: func(ctx context.Context, id int64) (db.ScheduleBlocking, error) {
			return db.ScheduleBlocking{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/schedule-blockings/404/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeactivateScheduleBlocking_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		GetScheduleBlockingByIDFunc: func(ctx context.Context, id int64) (db.ScheduleBlocking, error) {
			return db.ScheduleBlocking{ID: id, LabID: 1}, nil
		},
		DeactivateScheduleBlockingFunc: func(ctx context.Context, id int64) error {
			return assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/schedule-blockings/3/deactivate", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleDeactivateScheduleBlocking_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/schedule-blockings/not-a-number/deactivate", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
