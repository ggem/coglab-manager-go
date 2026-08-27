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

func TestHandleCreateEquipment_Success(t *testing.T) {
	var captured db.CreateEquipmentParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateEquipmentFunc: func(ctx context.Context, arg db.CreateEquipmentParams) (db.Equipment, error) {
			captured = arg
			return db.Equipment{ID: 1, LabID: arg.LabID, Name: arg.Name, Quantity: arg.Quantity}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/equipment/", cookie, equipmentRequest{Name: "Eye Tracker", Quantity: 2})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.LabID != 9 || captured.Name != "Eye Tracker" || captured.Quantity != 2 {
		t.Errorf("CreateEquipment params = %+v", captured)
	}
	got := decodeBody[equipmentResponse](t, rec)
	if got.ID != 1 || got.Quantity != 2 {
		t.Errorf("response = %+v", got)
	}
	if capturedAudit.Action != ActionEquipmentCreated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionEquipmentCreated)
	}
}

func TestHandleCreateEquipment_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodPost, "/labs/9/equipment/", nil, equipmentRequest{})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateEquipment_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/not-a-number/equipment/", cookie, equipmentRequest{Name: "X", Quantity: 1})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateEquipment_UnknownLab(t *testing.T) {
	q := &dbfake.Querier{
		CreateEquipmentFunc: func(ctx context.Context, arg db.CreateEquipmentParams) (db.Equipment, error) {
			return db.Equipment{}, &pgconn.PgError{Code: pgForeignKeyViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/404/equipment/", cookie, equipmentRequest{Name: "X", Quantity: 1})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// Negative quantity violates the equipment_quantity_check constraint added
// in migrations/20260817200726_add_equipment_quantity_check.sql; nothing
// in the handler validates it independently, so this exercises the same
// writeDBError check-violation path as an invalid enum value elsewhere.
func TestHandleCreateEquipment_NegativeQuantity(t *testing.T) {
	q := &dbfake.Querier{
		CreateEquipmentFunc: func(ctx context.Context, arg db.CreateEquipmentParams) (db.Equipment, error) {
			return db.Equipment{}, &pgconn.PgError{Code: pgCheckViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/equipment/", cookie, equipmentRequest{Name: "X", Quantity: -1})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateEquipment_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		CreateEquipmentFunc: func(ctx context.Context, arg db.CreateEquipmentParams) (db.Equipment, error) {
			return db.Equipment{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/equipment/", cookie, equipmentRequest{Name: "X", Quantity: 1})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleGetEquipment_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/equipment/not-a-number/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGetEquipment_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetEquipmentByIDFunc: func(ctx context.Context, id int64) (db.Equipment, error) {
			return db.Equipment{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/equipment/404/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleListEquipmentByLab_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/not-a-number/equipment/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateEquipment_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPut, "/equipment/not-a-number/", cookie, equipmentRequest{Name: "X", Quantity: 1})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateEquipment_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		UpdateEquipmentFunc: func(ctx context.Context, arg db.UpdateEquipmentParams) (db.Equipment, error) {
			return db.Equipment{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/equipment/404/", cookie, equipmentRequest{Name: "X", Quantity: 1})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateEquipment_NegativeQuantity(t *testing.T) {
	q := &dbfake.Querier{
		UpdateEquipmentFunc: func(ctx context.Context, arg db.UpdateEquipmentParams) (db.Equipment, error) {
			return db.Equipment{}, &pgconn.PgError{Code: pgCheckViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/equipment/3/", cookie, equipmentRequest{Name: "X", Quantity: -1})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateEquipment_Success(t *testing.T) {
	var captured db.UpdateEquipmentParams
	q := &dbfake.Querier{
		UpdateEquipmentFunc: func(ctx context.Context, arg db.UpdateEquipmentParams) (db.Equipment, error) {
			captured = arg
			return db.Equipment{ID: arg.ID, Name: arg.Name, Quantity: arg.Quantity}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/equipment/3/", cookie, equipmentRequest{Name: "Eye Tracker", Quantity: 5})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.ID != 3 || captured.Quantity != 5 {
		t.Errorf("UpdateEquipment params = %+v", captured)
	}
}

func TestHandleDeactivateEquipment_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/equipment/not-a-number/deactivate", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDeactivateEquipment_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		DeactivateEquipmentFunc: func(ctx context.Context, id int64) error {
			return pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/equipment/404/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeactivateEquipment_Success(t *testing.T) {
	var deactivatedID int64
	q := &dbfake.Querier{
		DeactivateEquipmentFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/equipment/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if deactivatedID != 3 {
		t.Errorf("deactivated ID = %d, want 3", deactivatedID)
	}
}
