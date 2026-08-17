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

func TestHandleCreateFamily_Success(t *testing.T) {
	var captured db.CreateFamilyParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateFamilyFunc: func(ctx context.Context, arg db.CreateFamilyParams) (db.Family, error) {
			captured = arg
			return db.Family{ID: 1, Address: arg.Address, City: arg.City, State: arg.State, Zip: arg.Zip}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/families/", cookie, familyRequest{Address: "1 Main St", City: "Boulder", State: "CO", Zip: "80301"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.Address != "1 Main St" {
		t.Errorf("CreateFamily Address = %q, want %q", captured.Address, "1 Main St")
	}
	got := decodeBody[familyResponse](t, rec)
	if got.ID != 1 {
		t.Errorf("response ID = %d, want 1", got.ID)
	}
	if capturedAudit.Action != ActionFamilyCreated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionFamilyCreated)
	}
	if capturedAudit.ActorUserID == nil || *capturedAudit.ActorUserID != 7 {
		t.Errorf("audit ActorUserID = %v, want 7", capturedAudit.ActorUserID)
	}
}

func TestHandleCreateFamily_InvalidContactMethod(t *testing.T) {
	q := &dbfake.Querier{
		CreateFamilyFunc: func(ctx context.Context, arg db.CreateFamilyParams) (db.Family, error) {
			return db.Family{}, &pgconn.PgError{Code: pgCheckViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	badMethod := "carrier_pigeon"
	rec := doRequest(t, s, http.MethodPost, "/families/", cookie, familyRequest{PreferredContactMethod: &badMethod})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateFamily_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodPost, "/families/", nil, familyRequest{})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleGetFamily_Success(t *testing.T) {
	q := &dbfake.Querier{
		GetFamilyByIDFunc: func(ctx context.Context, id int64) (db.Family, error) {
			return db.Family{ID: id, Address: "1 Main St"}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/families/1/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[familyResponse](t, rec)
	if got.ID != 1 || got.Address != "1 Main St" {
		t.Errorf("response = %+v", got)
	}
}

func TestHandleGetFamily_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetFamilyByIDFunc: func(ctx context.Context, id int64) (db.Family, error) {
			return db.Family{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/families/404/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleGetFamily_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/families/not-a-number/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateFamily_Success(t *testing.T) {
	var captured db.UpdateFamilyParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		UpdateFamilyFunc: func(ctx context.Context, arg db.UpdateFamilyParams) (db.Family, error) {
			captured = arg
			return db.Family{ID: arg.ID, Address: arg.Address}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/families/3/", cookie, familyRequest{Address: "2 Elm St"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.ID != 3 || captured.Address != "2 Elm St" {
		t.Errorf("UpdateFamily params = %+v", captured)
	}
	if capturedAudit.Action != ActionFamilyUpdated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionFamilyUpdated)
	}
}
