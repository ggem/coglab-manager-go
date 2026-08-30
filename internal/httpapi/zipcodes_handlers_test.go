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

func TestHandleCreateZipCode_Success(t *testing.T) {
	var captured db.CreateZipCodeParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateZipCodeFunc: func(ctx context.Context, arg db.CreateZipCodeParams) (db.Zipcode, error) {
			captured = arg
			return db.Zipcode{ID: 1, LabID: arg.LabID, ZipCode: arg.ZipCode, Priority: arg.Priority}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/zip-codes/", cookie, zipCodeRequest{ZipCode: "80301", Priority: "high"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.LabID != 9 || captured.ZipCode != "80301" || captured.Priority != "high" {
		t.Errorf("CreateZipCode params = %+v", captured)
	}
	got := decodeBody[zipCodeResponse](t, rec)
	if got.ID != 1 || got.ZipCode != "80301" {
		t.Errorf("response = %+v", got)
	}
	if capturedAudit.Action != ActionZipCodeCreated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionZipCodeCreated)
	}
}

func TestHandleCreateZipCode_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodPost, "/labs/9/zip-codes/", nil, zipCodeRequest{})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateZipCode_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/not-a-number/zip-codes/", cookie, zipCodeRequest{ZipCode: "80301"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateZipCode_UnknownLab(t *testing.T) {
	q := &dbfake.Querier{
		CreateZipCodeFunc: func(ctx context.Context, arg db.CreateZipCodeParams) (db.Zipcode, error) {
			return db.Zipcode{}, &pgconn.PgError{Code: pgForeignKeyViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/404/zip-codes/", cookie, zipCodeRequest{ZipCode: "80301"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateZipCode_AlreadyExists(t *testing.T) {
	// zipcodes_one_active_per_lab_and_zip is a partial unique index on
	// (lab_id, zip_code) among active rows.
	q := &dbfake.Querier{
		CreateZipCodeFunc: func(ctx context.Context, arg db.CreateZipCodeParams) (db.Zipcode, error) {
			return db.Zipcode{}, &pgconn.PgError{Code: pgUniqueViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/zip-codes/", cookie, zipCodeRequest{ZipCode: "80301"})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestHandleCreateZipCode_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		CreateZipCodeFunc: func(ctx context.Context, arg db.CreateZipCodeParams) (db.Zipcode, error) {
			return db.Zipcode{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/zip-codes/", cookie, zipCodeRequest{ZipCode: "80301"})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleGetZipCode_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/zip-codes/not-a-number/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGetZipCode_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetZipCodeByIDFunc: func(ctx context.Context, id int64) (db.Zipcode, error) {
			return db.Zipcode{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/zip-codes/404/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleListZipCodesByLab_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/not-a-number/zip-codes/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleListZipCodesByLab_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListZipCodesByLabFunc: func(ctx context.Context, labID int64) ([]db.Zipcode, error) {
			return []db.Zipcode{{ID: 1, LabID: labID, ZipCode: "80301"}, {ID: 2, LabID: labID, ZipCode: "80302"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/zip-codes/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]zipCodeResponse](t, rec)
	if len(got) != 2 {
		t.Errorf("len(response) = %d, want 2", len(got))
	}
}

func TestHandleUpdateZipCode_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPut, "/zip-codes/not-a-number/", cookie, zipCodeRequest{ZipCode: "80301"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateZipCode_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetZipCodeByIDFunc: func(ctx context.Context, id int64) (db.Zipcode, error) {
			return db.Zipcode{ID: id, LabID: 1}, nil
		},
		UpdateZipCodeFunc: func(ctx context.Context, arg db.UpdateZipCodeParams) (db.Zipcode, error) {
			return db.Zipcode{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/zip-codes/404/", cookie, zipCodeRequest{ZipCode: "80301"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateZipCode_Success(t *testing.T) {
	var captured db.UpdateZipCodeParams
	q := &dbfake.Querier{
		GetZipCodeByIDFunc: func(ctx context.Context, id int64) (db.Zipcode, error) {
			return db.Zipcode{ID: id, LabID: 1}, nil
		},
		UpdateZipCodeFunc: func(ctx context.Context, arg db.UpdateZipCodeParams) (db.Zipcode, error) {
			captured = arg
			return db.Zipcode{ID: arg.ID, ZipCode: arg.ZipCode, Priority: arg.Priority}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/zip-codes/3/", cookie, zipCodeRequest{ZipCode: "80303", Priority: "low"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.ID != 3 || captured.ZipCode != "80303" || captured.Priority != "low" {
		t.Errorf("UpdateZipCode params = %+v", captured)
	}
}

func TestHandleDeactivateZipCode_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/zip-codes/not-a-number/deactivate", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDeactivateZipCode_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetZipCodeByIDFunc: func(ctx context.Context, id int64) (db.Zipcode, error) {
			return db.Zipcode{ID: id, LabID: 1}, nil
		},
		DeactivateZipCodeFunc: func(ctx context.Context, id int64) error {
			return pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/zip-codes/404/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeactivateZipCode_Success(t *testing.T) {
	var deactivatedID int64
	q := &dbfake.Querier{
		GetZipCodeByIDFunc: func(ctx context.Context, id int64) (db.Zipcode, error) {
			return db.Zipcode{ID: id, LabID: 1}, nil
		},
		DeactivateZipCodeFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/zip-codes/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if deactivatedID != 3 {
		t.Errorf("deactivated ID = %d, want 3", deactivatedID)
	}
}
