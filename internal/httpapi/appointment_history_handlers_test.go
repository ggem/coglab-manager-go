package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleGetChildAppointmentHistory_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListAppointmentsByChildFunc: func(ctx context.Context, childID int64) ([]db.ListAppointmentsByChildRow, error) {
			return []db.ListAppointmentsByChildRow{
				{ID: 1, ExperimentName: "Looking Time Study", Status: "arrived"},
			}, nil
		},
		ListAppointmentsBySiblingsFunc: func(ctx context.Context, childID int64) ([]db.ListAppointmentsBySiblingsRow, error) {
			return []db.ListAppointmentsBySiblingsRow{
				{ID: 2, ExperimentName: "Word Learning Study", Status: "arrived", ChildFirstName: "Sib", ChildLastName: "Ling"},
			}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/children/9/appointment-history", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[childAppointmentHistoryResponse](t, rec)
	if len(got.Own) != 1 || got.Own[0].ExperimentName != "Looking Time Study" {
		t.Errorf("Own = %+v", got.Own)
	}
	if len(got.Siblings) != 1 || got.Siblings[0].ChildFirstName != "Sib" {
		t.Errorf("Siblings = %+v", got.Siblings)
	}
}

func TestHandleGetChildAppointmentHistory_InvalidChildID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/children/not-a-number/appointment-history", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGetChildAppointmentHistory_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		ListAppointmentsByChildFunc: func(ctx context.Context, childID int64) ([]db.ListAppointmentsByChildRow, error) {
			return nil, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/children/9/appointment-history", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
