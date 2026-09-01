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

func TestHandleCreateGuardian_Success(t *testing.T) {
	var captured db.CreateGuardianParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateGuardianFunc: func(ctx context.Context, arg db.CreateGuardianParams) (db.Guardian, error) {
			captured = arg
			return db.Guardian{ID: 5, FamilyID: arg.FamilyID, FirstName: arg.FirstName, LastName: arg.LastName}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/families/3/guardians/", cookie, guardianRequest{FirstName: "Pat", LastName: "Guardian"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.FamilyID != 3 {
		t.Errorf("CreateGuardian FamilyID = %d, want 3 (should come from the URL, not the body)", captured.FamilyID)
	}
	got := decodeBody[guardianResponse](t, rec)
	if got.ID != 5 {
		t.Errorf("response ID = %d, want 5", got.ID)
	}
	if capturedAudit.Action != ActionGuardianCreated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionGuardianCreated)
	}
}

func TestHandleCreateGuardian_UnknownFamily(t *testing.T) {
	q := &dbfake.Querier{
		CreateGuardianFunc: func(ctx context.Context, arg db.CreateGuardianParams) (db.Guardian, error) {
			return db.Guardian{}, &pgconn.PgError{Code: pgForeignKeyViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/families/999/guardians/", cookie, guardianRequest{FirstName: "Pat"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleListGuardiansByFamily_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListGuardiansByFamilyFunc: func(ctx context.Context, familyID int64) ([]db.Guardian, error) {
			return []db.Guardian{{ID: 1, FamilyID: familyID}, {ID: 2, FamilyID: familyID}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/families/3/guardians/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]guardianResponse](t, rec)
	if len(got) != 2 {
		t.Errorf("got %d guardians, want 2", len(got))
	}
}

func TestHandleGetGuardian_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetGuardianByIDFunc: func(ctx context.Context, id int64) (db.Guardian, error) {
			return db.Guardian{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/guardians/404/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateGuardian_Success(t *testing.T) {
	var captured db.UpdateGuardianParams
	q := &dbfake.Querier{
		UpdateGuardianFunc: func(ctx context.Context, arg db.UpdateGuardianParams) (db.Guardian, error) {
			captured = arg
			return db.Guardian{ID: arg.ID, FirstName: arg.FirstName}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/guardians/9/", cookie, guardianRequest{FirstName: "Updated"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.ID != 9 || captured.FirstName != "Updated" {
		t.Errorf("UpdateGuardian params = %+v", captured)
	}
}

func TestHandleDeactivateGuardian_Success(t *testing.T) {
	var deactivatedID int64
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		DeactivateGuardianFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodDelete, "/guardians/9/", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if deactivatedID != 9 {
		t.Errorf("DeactivateGuardian called with id = %d, want 9", deactivatedID)
	}
	if capturedAudit.Action != ActionGuardianDeactivated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionGuardianDeactivated)
	}
	if capturedAudit.EntityID == nil || *capturedAudit.EntityID != 9 {
		t.Errorf("audit EntityID = %v, want 9", capturedAudit.EntityID)
	}
}

func TestHandleDeactivateGuardian_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodDelete, "/guardians/9/", nil, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
