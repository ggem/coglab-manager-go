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

func TestHandleCreateGrant_Success(t *testing.T) {
	var captured db.CreateGrantParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateGrantFunc: func(ctx context.Context, arg db.CreateGrantParams) (db.Grant, error) {
			captured = arg
			return db.Grant{ID: 1, LabID: arg.LabID, Name: arg.Name}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/grants/", cookie, grantRequest{Name: "NIH-R01-12345"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.LabID != 9 || captured.Name != "NIH-R01-12345" {
		t.Errorf("CreateGrant params = %+v", captured)
	}
	got := decodeBody[grantResponse](t, rec)
	if got.ID != 1 {
		t.Errorf("response ID = %d, want 1", got.ID)
	}
	if capturedAudit.Action != ActionGrantCreated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionGrantCreated)
	}
}

func TestHandleCreateGrant_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodPost, "/labs/9/grants/", nil, grantRequest{})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateGrant_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/not-a-number/grants/", cookie, grantRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateGrant_UnknownLab(t *testing.T) {
	q := &dbfake.Querier{
		CreateGrantFunc: func(ctx context.Context, arg db.CreateGrantParams) (db.Grant, error) {
			return db.Grant{}, &pgconn.PgError{Code: pgForeignKeyViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/404/grants/", cookie, grantRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateGrant_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		CreateGrantFunc: func(ctx context.Context, arg db.CreateGrantParams) (db.Grant, error) {
			return db.Grant{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/grants/", cookie, grantRequest{Name: "X"})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleGetGrant_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/grants/not-a-number/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGetGrant_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetGrantByIDFunc: func(ctx context.Context, id int64) (db.Grant, error) {
			return db.Grant{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/grants/404/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleListGrantsByLab_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/not-a-number/grants/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleListGrantsByLab_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListGrantsByLabFunc: func(ctx context.Context, labID int64) ([]db.Grant, error) {
			return []db.Grant{{ID: 1, LabID: labID, Name: "A"}, {ID: 2, LabID: labID, Name: "B"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/grants/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]grantResponse](t, rec)
	if len(got) != 2 {
		t.Errorf("len(response) = %d, want 2", len(got))
	}
}

func TestHandleUpdateGrant_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPut, "/grants/not-a-number/", cookie, grantRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateGrant_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetGrantByIDFunc: func(ctx context.Context, id int64) (db.Grant, error) {
			return db.Grant{ID: id, LabID: 1}, nil
		},
		UpdateGrantFunc: func(ctx context.Context, arg db.UpdateGrantParams) (db.Grant, error) {
			return db.Grant{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/grants/404/", cookie, grantRequest{Name: "X"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateGrant_Success(t *testing.T) {
	var captured db.UpdateGrantParams
	q := &dbfake.Querier{
		GetGrantByIDFunc: func(ctx context.Context, id int64) (db.Grant, error) {
			return db.Grant{ID: id, LabID: 1}, nil
		},
		UpdateGrantFunc: func(ctx context.Context, arg db.UpdateGrantParams) (db.Grant, error) {
			captured = arg
			return db.Grant{ID: arg.ID, Name: arg.Name}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/grants/3/", cookie, grantRequest{Name: "Renamed"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.ID != 3 || captured.Name != "Renamed" {
		t.Errorf("UpdateGrant params = %+v", captured)
	}
}

func TestHandleDeactivateGrant_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/grants/not-a-number/deactivate", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDeactivateGrant_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetGrantByIDFunc: func(ctx context.Context, id int64) (db.Grant, error) {
			return db.Grant{ID: id, LabID: 1}, nil
		},
		DeactivateGrantFunc: func(ctx context.Context, id int64) error {
			return pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/grants/404/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeactivateGrant_Success(t *testing.T) {
	var deactivatedID int64
	q := &dbfake.Querier{
		GetGrantByIDFunc: func(ctx context.Context, id int64) (db.Grant, error) {
			return db.Grant{ID: id, LabID: 1}, nil
		},
		DeactivateGrantFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/grants/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if deactivatedID != 3 {
		t.Errorf("deactivated ID = %d, want 3", deactivatedID)
	}
}
