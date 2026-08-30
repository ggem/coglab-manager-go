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

func TestHandleCreateProtocol_Success(t *testing.T) {
	var captured db.CreateProtocolParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateProtocolFunc: func(ctx context.Context, arg db.CreateProtocolParams) (db.Protocol, error) {
			captured = arg
			return db.Protocol{ID: 1, LabID: arg.LabID, Name: arg.Name}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/protocols/", cookie, protocolRequest{Name: "IRB-001"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.LabID != 9 || captured.Name != "IRB-001" {
		t.Errorf("CreateProtocol params = %+v", captured)
	}
	got := decodeBody[protocolResponse](t, rec)
	if got.ID != 1 {
		t.Errorf("response ID = %d, want 1", got.ID)
	}
	if capturedAudit.Action != ActionProtocolCreated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionProtocolCreated)
	}
}

func TestHandleCreateProtocol_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodPost, "/labs/9/protocols/", nil, protocolRequest{})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateProtocol_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/not-a-number/protocols/", cookie, protocolRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateProtocol_UnknownLab(t *testing.T) {
	q := &dbfake.Querier{
		CreateProtocolFunc: func(ctx context.Context, arg db.CreateProtocolParams) (db.Protocol, error) {
			return db.Protocol{}, &pgconn.PgError{Code: pgForeignKeyViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/404/protocols/", cookie, protocolRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateProtocol_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		CreateProtocolFunc: func(ctx context.Context, arg db.CreateProtocolParams) (db.Protocol, error) {
			return db.Protocol{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/protocols/", cookie, protocolRequest{Name: "X"})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleGetProtocol_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/protocols/not-a-number/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGetProtocol_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetProtocolByIDFunc: func(ctx context.Context, id int64) (db.Protocol, error) {
			return db.Protocol{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/protocols/404/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleListProtocolsByLab_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/not-a-number/protocols/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleListProtocolsByLab_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListProtocolsByLabFunc: func(ctx context.Context, labID int64) ([]db.Protocol, error) {
			return []db.Protocol{{ID: 1, LabID: labID, Name: "A"}, {ID: 2, LabID: labID, Name: "B"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/protocols/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]protocolResponse](t, rec)
	if len(got) != 2 {
		t.Errorf("len(response) = %d, want 2", len(got))
	}
}

func TestHandleUpdateProtocol_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPut, "/protocols/not-a-number/", cookie, protocolRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateProtocol_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetProtocolByIDFunc: func(ctx context.Context, id int64) (db.Protocol, error) {
			return db.Protocol{ID: id, LabID: 1}, nil
		},
		UpdateProtocolFunc: func(ctx context.Context, arg db.UpdateProtocolParams) (db.Protocol, error) {
			return db.Protocol{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/protocols/404/", cookie, protocolRequest{Name: "X"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateProtocol_Success(t *testing.T) {
	var captured db.UpdateProtocolParams
	q := &dbfake.Querier{
		GetProtocolByIDFunc: func(ctx context.Context, id int64) (db.Protocol, error) {
			return db.Protocol{ID: id, LabID: 1}, nil
		},
		UpdateProtocolFunc: func(ctx context.Context, arg db.UpdateProtocolParams) (db.Protocol, error) {
			captured = arg
			return db.Protocol{ID: arg.ID, Name: arg.Name}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/protocols/3/", cookie, protocolRequest{Name: "Renamed"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.ID != 3 || captured.Name != "Renamed" {
		t.Errorf("UpdateProtocol params = %+v", captured)
	}
}

func TestHandleDeactivateProtocol_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/protocols/not-a-number/deactivate", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDeactivateProtocol_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetProtocolByIDFunc: func(ctx context.Context, id int64) (db.Protocol, error) {
			return db.Protocol{ID: id, LabID: 1}, nil
		},
		DeactivateProtocolFunc: func(ctx context.Context, id int64) error {
			return pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/protocols/404/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeactivateProtocol_Success(t *testing.T) {
	var deactivatedID int64
	q := &dbfake.Querier{
		GetProtocolByIDFunc: func(ctx context.Context, id int64) (db.Protocol, error) {
			return db.Protocol{ID: id, LabID: 1}, nil
		},
		DeactivateProtocolFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/protocols/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if deactivatedID != 3 {
		t.Errorf("deactivated ID = %d, want 3", deactivatedID)
	}
}
