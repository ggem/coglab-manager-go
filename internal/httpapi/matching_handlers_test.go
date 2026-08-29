package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func birthDate(t *testing.T, s string) pgtype.Date {
	t.Helper()
	parsed, err := time.Parse(dateLayout, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return pgtype.Date{Time: parsed, Valid: true}
}

func TestHandleHoldChildrenForExperiment_Success(t *testing.T) {
	var created []db.CreateAppointmentParams
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		ListEligibleChildrenForExperimentFunc: func(ctx context.Context, arg db.ListEligibleChildrenForExperimentParams) ([]db.Child, error) {
			return []db.Child{
				{ID: 1, BirthDate: birthDate(t, "2024-01-01")},
				{ID: 2, BirthDate: birthDate(t, "2023-01-01")},
			}, nil
		},
		CreateAppointmentFunc: func(ctx context.Context, arg db.CreateAppointmentParams) (db.Appointment, error) {
			created = append(created, arg)
			return db.Appointment{ID: int64(len(created)), ExperimentID: arg.ExperimentID, ChildID: arg.ChildID, Status: "to_be_scheduled"}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/5/hold-children", cookie, holdChildrenRequest{
		StartDate: "2026-01-01", EndDate: "2026-02-01", Count: 2, Sort: "oldest",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]appointmentResponse](t, rec)
	if len(got) != 2 {
		t.Fatalf("held %d children, want 2", len(got))
	}
	// Oldest-first: child 2 (born 2023) should be picked before child 1 (born 2024).
	if created[0].ChildID != 2 || created[1].ChildID != 1 {
		t.Errorf("hold order = %+v, want child 2 then child 1", created)
	}
}

func TestHandleHoldChildrenForExperiment_LimitsToRequestedCount(t *testing.T) {
	var created []db.CreateAppointmentParams
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		ListEligibleChildrenForExperimentFunc: func(ctx context.Context, arg db.ListEligibleChildrenForExperimentParams) ([]db.Child, error) {
			return []db.Child{
				{ID: 1, BirthDate: birthDate(t, "2024-01-01")},
				{ID: 2, BirthDate: birthDate(t, "2023-01-01")},
				{ID: 3, BirthDate: birthDate(t, "2022-01-01")},
			}, nil
		},
		CreateAppointmentFunc: func(ctx context.Context, arg db.CreateAppointmentParams) (db.Appointment, error) {
			created = append(created, arg)
			return db.Appointment{ID: int64(len(created)), ExperimentID: arg.ExperimentID, ChildID: arg.ChildID}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/5/hold-children", cookie, holdChildrenRequest{
		StartDate: "2026-01-01", EndDate: "2026-02-01", Count: 1, Sort: "oldest",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if len(created) != 1 {
		t.Fatalf("created %d appointments, want 1 (count cap)", len(created))
	}
}

// TestHandleHoldChildrenForExperiment_SkipsRaceLoser confirms a unique
// violation on one candidate (another concurrent hold-children call beat
// this one to that child) is skipped, not treated as a request failure --
// the response still succeeds with whichever candidates were actually
// held.
func TestHandleHoldChildrenForExperiment_SkipsRaceLoser(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		ListEligibleChildrenForExperimentFunc: func(ctx context.Context, arg db.ListEligibleChildrenForExperimentParams) ([]db.Child, error) {
			return []db.Child{
				{ID: 1, BirthDate: birthDate(t, "2024-01-01")},
				{ID: 2, BirthDate: birthDate(t, "2023-01-01")},
			}, nil
		},
		CreateAppointmentFunc: func(ctx context.Context, arg db.CreateAppointmentParams) (db.Appointment, error) {
			if arg.ChildID == 2 {
				return db.Appointment{}, &pgconn.PgError{Code: pgUniqueViolation}
			}
			return db.Appointment{ID: 1, ExperimentID: arg.ExperimentID, ChildID: arg.ChildID}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/5/hold-children", cookie, holdChildrenRequest{
		StartDate: "2026-01-01", EndDate: "2026-02-01", Count: 2, Sort: "oldest",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]appointmentResponse](t, rec)
	if len(got) != 1 || got[0].ChildID != 1 {
		t.Fatalf("held = %+v, want exactly child 1 (child 2 raced away)", got)
	}
}

func TestHandleHoldChildrenForExperiment_InvalidCount(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/5/hold-children", cookie, holdChildrenRequest{
		StartDate: "2026-01-01", EndDate: "2026-02-01", Count: 0,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleHoldChildrenForExperiment_InvalidSort(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/5/hold-children", cookie, holdChildrenRequest{
		StartDate: "2026-01-01", EndDate: "2026-02-01", Count: 1, Sort: "shuffle",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleHoldChildrenForExperiment_InvalidStartDate(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/5/hold-children", cookie, holdChildrenRequest{
		StartDate: "not-a-date", EndDate: "2026-02-01", Count: 1,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleHoldChildrenForExperiment_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		ListEligibleChildrenForExperimentFunc: func(ctx context.Context, arg db.ListEligibleChildrenForExperimentParams) ([]db.Child, error) {
			return nil, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/5/hold-children", cookie, holdChildrenRequest{
		StartDate: "2026-01-01", EndDate: "2026-02-01", Count: 1,
	})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleHoldChildrenForExperiment_CreateAppointmentUnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		ListEligibleChildrenForExperimentFunc: func(ctx context.Context, arg db.ListEligibleChildrenForExperimentParams) ([]db.Child, error) {
			return []db.Child{{ID: 1, BirthDate: birthDate(t, "2024-01-01")}}, nil
		},
		CreateAppointmentFunc: func(ctx context.Context, arg db.CreateAppointmentParams) (db.Appointment, error) {
			return db.Appointment{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/5/hold-children", cookie, holdChildrenRequest{
		StartDate: "2026-01-01", EndDate: "2026-02-01", Count: 1,
	})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
