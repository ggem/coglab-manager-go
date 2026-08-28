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

func TestHandleCreateExperimentRole_Success(t *testing.T) {
	var captured db.CreateExperimentRoleParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateExperimentRoleFunc: func(ctx context.Context, arg db.CreateExperimentRoleParams) (db.ExperimentRole, error) {
			captured = arg
			return db.ExperimentRole{ID: 1, LabID: arg.LabID, Name: arg.Name}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/experiment-roles/", cookie, experimentRoleRequest{Name: "Experimenter"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.LabID != 9 || captured.Name != "Experimenter" {
		t.Errorf("CreateExperimentRole params = %+v", captured)
	}
	if capturedAudit.Action != ActionExperimentRoleCreated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionExperimentRoleCreated)
	}
}

func TestHandleCreateExperimentRole_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodPost, "/labs/9/experiment-roles/", nil, experimentRoleRequest{})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateExperimentRole_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/not-a-number/experiment-roles/", cookie, experimentRoleRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateExperimentRole_UnknownLab(t *testing.T) {
	q := &dbfake.Querier{
		CreateExperimentRoleFunc: func(ctx context.Context, arg db.CreateExperimentRoleParams) (db.ExperimentRole, error) {
			return db.ExperimentRole{}, &pgconn.PgError{Code: pgForeignKeyViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/404/experiment-roles/", cookie, experimentRoleRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateExperimentRole_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		CreateExperimentRoleFunc: func(ctx context.Context, arg db.CreateExperimentRoleParams) (db.ExperimentRole, error) {
			return db.ExperimentRole{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/experiment-roles/", cookie, experimentRoleRequest{Name: "X"})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleGetExperimentRole_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiment-roles/not-a-number/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGetExperimentRole_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentRoleByIDFunc: func(ctx context.Context, id int64) (db.ExperimentRole, error) {
			return db.ExperimentRole{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiment-roles/404/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleListExperimentRolesByLab_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/not-a-number/experiment-roles/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateExperimentRole_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPut, "/experiment-roles/not-a-number/", cookie, experimentRoleRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateExperimentRole_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentRoleByIDFunc: func(ctx context.Context, id int64) (db.ExperimentRole, error) {
			return db.ExperimentRole{ID: id, LabID: 1}, nil
		},
		UpdateExperimentRoleFunc: func(ctx context.Context, arg db.UpdateExperimentRoleParams) (db.ExperimentRole, error) {
			return db.ExperimentRole{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/experiment-roles/404/", cookie, experimentRoleRequest{Name: "X"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleListExperimentRolesByLab_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListExperimentRolesByLabFunc: func(ctx context.Context, labID int64) ([]db.ExperimentRole, error) {
			return []db.ExperimentRole{{ID: 1, LabID: labID, Name: "Experimenter"}, {ID: 2, LabID: labID, Name: "Coder"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/experiment-roles/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]experimentRoleResponse](t, rec)
	if len(got) != 2 {
		t.Errorf("len(response) = %d, want 2", len(got))
	}
}

func TestHandleDeactivateExperimentRole_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiment-roles/not-a-number/deactivate", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDeactivateExperimentRole_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentRoleByIDFunc: func(ctx context.Context, id int64) (db.ExperimentRole, error) {
			return db.ExperimentRole{ID: id, LabID: 1}, nil
		},
		DeactivateExperimentRoleFunc: func(ctx context.Context, id int64) error {
			return pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiment-roles/404/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeactivateExperimentRole_Success(t *testing.T) {
	var deactivatedID int64
	q := &dbfake.Querier{
		GetExperimentRoleByIDFunc: func(ctx context.Context, id int64) (db.ExperimentRole, error) {
			return db.ExperimentRole{ID: id, LabID: 1}, nil
		},
		DeactivateExperimentRoleFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiment-roles/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if deactivatedID != 3 {
		t.Errorf("deactivated ID = %d, want 3", deactivatedID)
	}
}

func TestHandleSetExperimentRoleSitter_Success(t *testing.T) {
	var captured db.SetExperimentRoleSitterParams
	q := &dbfake.Querier{
		GetExperimentRoleByIDFunc: func(ctx context.Context, id int64) (db.ExperimentRole, error) {
			return db.ExperimentRole{ID: id, LabID: 1}, nil
		},
		SetExperimentRoleSitterFunc: func(ctx context.Context, arg db.SetExperimentRoleSitterParams) (db.ExperimentRole, error) {
			captured = arg
			return db.ExperimentRole{ID: arg.ID, LabID: 1, IsSitterRole: arg.IsSitterRole}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiment-roles/3/set-sitter", cookie, setExperimentRoleSitterRequest{IsSitterRole: true})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.ID != 3 || !captured.IsSitterRole {
		t.Errorf("SetExperimentRoleSitter params = %+v", captured)
	}
	got := decodeBody[experimentRoleResponse](t, rec)
	if !got.IsSitterRole {
		t.Errorf("response IsSitterRole = false, want true")
	}
}

func TestHandleSetExperimentRoleSitter_AlreadyOneInLab(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentRoleByIDFunc: func(ctx context.Context, id int64) (db.ExperimentRole, error) {
			return db.ExperimentRole{ID: id, LabID: 1}, nil
		},
		SetExperimentRoleSitterFunc: func(ctx context.Context, arg db.SetExperimentRoleSitterParams) (db.ExperimentRole, error) {
			return db.ExperimentRole{}, &pgconn.PgError{Code: pgUniqueViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiment-roles/3/set-sitter", cookie, setExperimentRoleSitterRequest{IsSitterRole: true})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestHandleSetExperimentRoleSitter_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiment-roles/not-a-number/set-sitter", cookie, setExperimentRoleSitterRequest{IsSitterRole: true})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
