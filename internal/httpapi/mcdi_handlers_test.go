package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
	"github.com/ggem/coglab-manager-go/internal/mcdi"
	"github.com/ggem/coglab-manager-go/internal/mcdi/mcdifake"
)

func TestHandleRequestMCDI_Success(t *testing.T) {
	bday := birthDate(t, "2024-01-15")
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		GetChildByIDFunc: func(ctx context.Context, id int64) (db.Child, error) {
			return db.Child{ID: id, FamilyID: 3, FirstName: "Sam", LastName: "Smith", Sex: "female", BirthDate: bday}, nil
		},
		ListGuardiansByFamilyFunc: func(ctx context.Context, familyID int64) ([]db.Guardian, error) {
			return []db.Guardian{
				{ID: 1, FamilyID: familyID, FirstName: "Parent", LastName: "One", Email: "parent@example.edu"},
			}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	mcdiClient := &mcdifake.Client{}
	s, cookie := newAuthenticatedTestServerWithMCDI(q, 7, mcdiClient)

	rec := doRequest(t, s, http.MethodPost, "/children/5/request-mcdi", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}

	sent := mcdiClient.Sent()
	if len(sent) != 1 {
		t.Fatalf("Sent() = %+v, want exactly one request", sent)
	}
	want := mcdi.Request{ChildName: "Sam Smith", ParentEmail: "parent@example.edu", Gender: "female", Birthday: "2024-01-15", DatabaseID: 5}
	if sent[0] != want {
		t.Errorf("request = %+v, want %+v", sent[0], want)
	}

	if capturedAudit.Action != ActionMCDIRequested {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionMCDIRequested)
	}
}

func TestHandleRequestMCDI_ChildNotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetChildByIDFunc: func(ctx context.Context, id int64) (db.Child, error) {
			return db.Child{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/children/404/request-mcdi", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleRequestMCDI_NoBirthDate(t *testing.T) {
	q := &dbfake.Querier{
		GetChildByIDFunc: func(ctx context.Context, id int64) (db.Child, error) {
			return db.Child{ID: id, FamilyID: 3}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/children/5/request-mcdi", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleRequestMCDI_NoGuardianEmail(t *testing.T) {
	bday := birthDate(t, "2024-01-15")
	q := &dbfake.Querier{
		GetChildByIDFunc: func(ctx context.Context, id int64) (db.Child, error) {
			return db.Child{ID: id, FamilyID: 3, BirthDate: bday}, nil
		},
		ListGuardiansByFamilyFunc: func(ctx context.Context, familyID int64) ([]db.Guardian, error) {
			return nil, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/children/5/request-mcdi", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleRequestMCDI_GuardianWithBlankEmail(t *testing.T) {
	bday := birthDate(t, "2024-01-15")
	q := &dbfake.Querier{
		GetChildByIDFunc: func(ctx context.Context, id int64) (db.Child, error) {
			return db.Child{ID: id, FamilyID: 3, BirthDate: bday}, nil
		},
		ListGuardiansByFamilyFunc: func(ctx context.Context, familyID int64) ([]db.Guardian, error) {
			return []db.Guardian{{ID: 1, FamilyID: familyID, Email: ""}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/children/5/request-mcdi", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleRequestMCDI_UpstreamFailure(t *testing.T) {
	bday := birthDate(t, "2024-01-15")
	q := &dbfake.Querier{
		GetChildByIDFunc: func(ctx context.Context, id int64) (db.Child, error) {
			return db.Child{ID: id, FamilyID: 3, BirthDate: bday}, nil
		},
		ListGuardiansByFamilyFunc: func(ctx context.Context, familyID int64) ([]db.Guardian, error) {
			return []db.Guardian{{ID: 1, FamilyID: familyID, Email: "parent@example.edu"}}, nil
		},
	}
	mcdiClient := &mcdifake.Client{Err: errors.New("daxlabbase unreachable")}
	s, cookie := newAuthenticatedTestServerWithMCDI(q, 7, mcdiClient)

	rec := doRequest(t, s, http.MethodPost, "/children/5/request-mcdi", cookie, nil)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadGateway, rec.Body)
	}
}
