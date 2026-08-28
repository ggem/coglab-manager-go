package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func withRole(q *dbfake.Querier, roleID, labID int64) {
	q.GetExperimentRoleByIDFunc = func(ctx context.Context, id int64) (db.ExperimentRole, error) {
		return db.ExperimentRole{ID: id, LabID: labID}, nil
	}
}

func TestHandleAddLabMemberTraining_Success(t *testing.T) {
	var captured db.AddLabMemberTrainingParams
	q := &dbfake.Querier{
		AddLabMemberTrainingFunc: func(ctx context.Context, arg db.AddLabMemberTrainingParams) error {
			captured = arg
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	withRole(q, 3, 1)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiment-roles/3/trainings/", cookie, addLabMemberTrainingRequest{UserID: 42})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.UserID != 42 || captured.ExperimentRoleID != 3 {
		t.Errorf("AddLabMemberTraining params = %+v", captured)
	}
}

func TestHandleAddLabMemberTraining_UnknownUser(t *testing.T) {
	q := &dbfake.Querier{
		AddLabMemberTrainingFunc: func(ctx context.Context, arg db.AddLabMemberTrainingParams) error {
			return &pgconn.PgError{Code: pgForeignKeyViolation}
		},
	}
	withRole(q, 3, 1)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiment-roles/3/trainings/", cookie, addLabMemberTrainingRequest{UserID: 404})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleRemoveLabMemberTraining_Success(t *testing.T) {
	var captured db.RemoveLabMemberTrainingParams
	q := &dbfake.Querier{
		RemoveLabMemberTrainingFunc: func(ctx context.Context, arg db.RemoveLabMemberTrainingParams) error {
			captured = arg
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	withRole(q, 3, 1)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodDelete, "/experiment-roles/3/trainings/42", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.UserID != 42 || captured.ExperimentRoleID != 3 {
		t.Errorf("RemoveLabMemberTraining params = %+v", captured)
	}
}

func TestHandleRemoveLabMemberTraining_InvalidUserID(t *testing.T) {
	q := &dbfake.Querier{}
	withRole(q, 3, 1)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodDelete, "/experiment-roles/3/trainings/not-a-number", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleListLabMemberTrainingsForRole_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListLabMemberTrainingsForRoleFunc: func(ctx context.Context, experimentRoleID int64) ([]db.User, error) {
			return []db.User{{ID: 42, FirstName: "Ada", LastName: "Lovelace"}}, nil
		},
	}
	withRole(q, 3, 1)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiment-roles/3/trainings/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]trainedMemberResponse](t, rec)
	if len(got) != 1 || got[0].ID != 42 || got[0].FirstName != "Ada" {
		t.Errorf("response = %+v", got)
	}
}
