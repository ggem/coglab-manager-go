package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleCreateLabAvailabilityGeneral_Success(t *testing.T) {
	var captured db.CreateLabAvailabilityGeneralParams
	q := &dbfake.Querier{
		CreateLabAvailabilityGeneralFunc: func(ctx context.Context, arg db.CreateLabAvailabilityGeneralParams) (db.LabAvailabilityGeneral, error) {
			captured = arg
			return db.LabAvailabilityGeneral{ID: 1, UserID: arg.UserID, LabID: arg.LabID, Weekday: arg.Weekday, StartTime: arg.StartTime, EndTime: arg.EndTime}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/availability/general/", cookie, labAvailabilityGeneralRequest{
		Weekday: 1, StartTime: "09:00", EndTime: "17:00",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.UserID != 7 || captured.LabID != 9 || captured.Weekday != 1 {
		t.Errorf("CreateLabAvailabilityGeneral params = %+v", captured)
	}
	got := decodeBody[labAvailabilityGeneralResponse](t, rec)
	if got.StartTime != "09:00" || got.EndTime != "17:00" {
		t.Errorf("response = %+v", got)
	}
}

func TestHandleCreateLabAvailabilityGeneral_InvalidStartTime(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/availability/general/", cookie, labAvailabilityGeneralRequest{
		Weekday: 1, StartTime: "not-a-time", EndTime: "17:00",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleListLabAvailabilityGeneral_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListLabAvailabilityGeneralByUserFunc: func(ctx context.Context, arg db.ListLabAvailabilityGeneralByUserParams) ([]db.LabAvailabilityGeneral, error) {
			if arg.UserID != 7 {
				t.Errorf("ListLabAvailabilityGeneralByUser called with UserID=%d, want 7 (the current user)", arg.UserID)
			}
			return []db.LabAvailabilityGeneral{{ID: 1, UserID: 7, LabID: 9}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/availability/general/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
}

func TestHandleDeactivateLabAvailabilityGeneral_Success(t *testing.T) {
	var deactivatedID int64
	q := &dbfake.Querier{
		GetLabAvailabilityGeneralByIDFunc: func(ctx context.Context, id int64) (db.LabAvailabilityGeneral, error) {
			return db.LabAvailabilityGeneral{ID: id, UserID: 7, LabID: 1}, nil
		},
		DeactivateLabAvailabilityGeneralFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/availability/general/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if deactivatedID != 3 {
		t.Errorf("deactivated ID = %d, want 3", deactivatedID)
	}
}

func TestHandleDeactivateLabAvailabilityGeneral_NotOwnedByCurrentUser(t *testing.T) {
	q := &dbfake.Querier{
		GetLabAvailabilityGeneralByIDFunc: func(ctx context.Context, id int64) (db.LabAvailabilityGeneral, error) {
			return db.LabAvailabilityGeneral{ID: id, UserID: 999, LabID: 1}, nil // belongs to someone else
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/availability/general/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d -- another user's row should look like it doesn't exist", rec.Code, http.StatusNotFound)
	}
}

func TestHandleCreateLabAvailabilitySpecific_Success(t *testing.T) {
	var captured db.CreateLabAvailabilitySpecificParams
	q := &dbfake.Querier{
		CreateLabAvailabilitySpecificFunc: func(ctx context.Context, arg db.CreateLabAvailabilitySpecificParams) (db.LabAvailabilitySpecific, error) {
			captured = arg
			return db.LabAvailabilitySpecific{ID: 1, UserID: arg.UserID, LabID: arg.LabID, Date: arg.Date, StartTime: arg.StartTime, EndTime: arg.EndTime}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/availability/specific/", cookie, labAvailabilitySpecificRequest{
		Date: "2026-09-01", StartTime: "10:00", EndTime: "12:00",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.UserID != 7 || captured.LabID != 9 {
		t.Errorf("CreateLabAvailabilitySpecific params = %+v", captured)
	}
}

func TestHandleCreateLabAvailabilitySpecific_InvalidDate(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/availability/specific/", cookie, labAvailabilitySpecificRequest{
		Date: "not-a-date", StartTime: "10:00", EndTime: "12:00",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDeactivateLabAvailabilitySpecific_NotOwnedByCurrentUser(t *testing.T) {
	q := &dbfake.Querier{
		GetLabAvailabilitySpecificByIDFunc: func(ctx context.Context, id int64) (db.LabAvailabilitySpecific, error) {
			return db.LabAvailabilitySpecific{ID: id, UserID: 999, LabID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/availability/specific/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
