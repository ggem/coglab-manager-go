package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleCreateChild_Success(t *testing.T) {
	var captured db.CreateChildParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateChildFunc: func(ctx context.Context, arg db.CreateChildParams) (db.Child, error) {
			captured = arg
			return db.Child{ID: 1, FamilyID: arg.FamilyID, FirstName: arg.FirstName, BirthDate: arg.BirthDate}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	birthDate := "2024-01-15"
	rec := doRequest(t, s, http.MethodPost, "/families/3/children/", cookie, childRequest{
		FirstName: "Sam", LastName: "Smith", Sex: "unknown", BirthDate: &birthDate,
		RaceEthnicity: []string{"white"}, Languages: []string{"english"}, Response: "unknown",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.FamilyID != 3 {
		t.Errorf("CreateChild FamilyID = %d, want 3", captured.FamilyID)
	}
	if captured.CreatedByUserID != 7 {
		t.Errorf("CreateChild CreatedByUserID = %d, want 7 (from the session)", captured.CreatedByUserID)
	}
	if !captured.BirthDate.Valid || captured.BirthDate.Time.Format(dateLayout) != birthDate {
		t.Errorf("CreateChild BirthDate = %+v, want %s", captured.BirthDate, birthDate)
	}
	got := decodeBody[childResponse](t, rec)
	if got.BirthDate == nil || *got.BirthDate != birthDate {
		t.Errorf("response BirthDate = %v, want %q", got.BirthDate, birthDate)
	}
	if capturedAudit.Action != ActionChildCreated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionChildCreated)
	}
}

func TestHandleCreateChild_InvalidBirthDate(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	badDate := "not-a-date"
	rec := doRequest(t, s, http.MethodPost, "/families/3/children/", cookie, childRequest{BirthDate: &badDate})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGetChild_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetChildByIDFunc: func(ctx context.Context, id int64) (db.Child, error) {
			return db.Child{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/children/404/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleListChildrenByFamily_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListChildrenByFamilyFunc: func(ctx context.Context, familyID int64) ([]db.Child, error) {
			return []db.Child{{ID: 1, FamilyID: familyID}, {ID: 2, FamilyID: familyID}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/families/3/children/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]childResponse](t, rec)
	if len(got) != 2 {
		t.Errorf("got %d children, want 2", len(got))
	}
}

func TestHandleUpdateChild_Success(t *testing.T) {
	var captured db.UpdateChildParams
	q := &dbfake.Querier{
		UpdateChildFunc: func(ctx context.Context, arg db.UpdateChildParams) (db.Child, error) {
			captured = arg
			return db.Child{ID: arg.ID, FirstName: arg.FirstName}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/children/9/", cookie, childRequest{FirstName: "Updated", RaceEthnicity: []string{}, Languages: []string{}})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.ID != 9 || captured.FirstName != "Updated" {
		t.Errorf("UpdateChild params = %+v", captured)
	}
}

func TestHandleDeactivateChild_Success(t *testing.T) {
	var captured db.DeactivateChildParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		DeactivateChildFunc: func(ctx context.Context, arg db.DeactivateChildParams) error {
			captured = arg
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/children/9/deactivate", cookie, deactivateChildRequest{Reason: "moved away"})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.ID != 9 || captured.InactiveReason != "moved away" {
		t.Errorf("DeactivateChild params = %+v", captured)
	}
	if capturedAudit.Action != ActionChildDeactivated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionChildDeactivated)
	}
}
