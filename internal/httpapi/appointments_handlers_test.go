package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleCreateAppointment_Success(t *testing.T) {
	var captured db.CreateAppointmentParams
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		CreateAppointmentFunc: func(ctx context.Context, arg db.CreateAppointmentParams) (db.Appointment, error) {
			captured = arg
			return db.Appointment{ID: 1, ExperimentID: arg.ExperimentID, ChildID: arg.ChildID, Session: arg.Session, SiblingComing: arg.SiblingComing, Status: "to_be_scheduled"}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/5/appointments", cookie, appointmentRequest{ChildID: 10})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.ExperimentID != 5 || captured.ChildID != 10 {
		t.Errorf("CreateAppointment params = %+v", captured)
	}
	if captured.Session != 1 {
		t.Errorf("Session = %d, want 1 (default)", captured.Session)
	}
	if captured.SiblingComing != "unknown" {
		t.Errorf("SiblingComing = %q, want \"unknown\" (default)", captured.SiblingComing)
	}
	got := decodeBody[appointmentResponse](t, rec)
	if got.Status != "to_be_scheduled" {
		t.Errorf("response Status = %q, want \"to_be_scheduled\"", got.Status)
	}
}

func TestHandleCreateAppointment_UnknownChild(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		CreateAppointmentFunc: func(ctx context.Context, arg db.CreateAppointmentParams) (db.Appointment, error) {
			return db.Appointment{}, &pgconn.PgError{Code: pgForeignKeyViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/5/appointments", cookie, appointmentRequest{ChildID: 404})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateAppointment_InvalidExperimentID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/not-a-number/appointments", cookie, appointmentRequest{ChildID: 10})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// minimalSearchQuerier stubs every query buildAvailabilitySearch needs for
// a lab with no roles, no equipment, and no sitter role configured -- the
// simplest case where the search should trivially succeed for most of the
// day (an empty role list is satisfiable, per internal/scheduling).
func minimalSearchQuerier(experiment db.Experiment, appointment db.Appointment) *dbfake.Querier {
	return &dbfake.Querier{
		GetAppointmentByIDFunc: func(ctx context.Context, id int64) (db.Appointment, error) {
			return appointment, nil
		},
		GetAppointmentLabIDFunc: func(ctx context.Context, id int64) (int64, error) {
			return experiment.LabID, nil
		},
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return experiment, nil
		},
		ListExperimentTrainingRequirementsFunc: func(ctx context.Context, experimentID int64) ([]db.ExperimentRole, error) {
			return nil, nil
		},
		GetSitterRoleForLabFunc: func(ctx context.Context, labID int64) (db.ExperimentRole, error) {
			return db.ExperimentRole{}, pgx.ErrNoRows
		},
		ListExperimentEquipmentFunc: func(ctx context.Context, experimentID int64) ([]db.Equipment, error) {
			return nil, nil
		},
		ListLabAvailabilityGeneralByLabFunc: func(ctx context.Context, labID int64) ([]db.LabAvailabilityGeneral, error) {
			return nil, nil
		},
		ListLabAvailabilitySpecificForDateRangeFunc: func(ctx context.Context, arg db.ListLabAvailabilitySpecificForDateRangeParams) ([]db.LabAvailabilitySpecific, error) {
			return nil, nil
		},
		ListBusyAppointmentExperimentersForDateRangeFunc: func(ctx context.Context, arg db.ListBusyAppointmentExperimentersForDateRangeParams) ([]db.ListBusyAppointmentExperimentersForDateRangeRow, error) {
			return nil, nil
		},
		ListBusyEquipmentForDateRangeFunc: func(ctx context.Context, arg db.ListBusyEquipmentForDateRangeParams) ([]db.ListBusyEquipmentForDateRangeRow, error) {
			return nil, nil
		},
		ListScheduleBlockingsForDateRangeFunc: func(ctx context.Context, arg db.ListScheduleBlockingsForDateRangeParams) ([]db.ScheduleBlocking, error) {
			return nil, nil
		},
	}
}

func TestHandleSearchAppointmentAvailability_Success(t *testing.T) {
	experiment := db.Experiment{ID: 5, LabID: 1, DurationMinutes: 30}
	appointment := db.Appointment{ID: 3, ExperimentID: 5, SiblingComing: "unknown"}
	q := minimalSearchQuerier(experiment, appointment)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/appointments/3/availability?start_date=2026-09-01&end_date=2026-09-01", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]candidateSlotResponse](t, rec)
	if len(got) == 0 {
		t.Fatal("expected candidate slots with no roles/equipment/blockings constraining the day")
	}
	if got[0].Date != "2026-09-01" {
		t.Errorf("Date = %q, want 2026-09-01", got[0].Date)
	}
}

func TestHandleSearchAppointmentAvailability_MissingDates(t *testing.T) {
	q := minimalSearchQuerier(db.Experiment{ID: 5, LabID: 1}, db.Appointment{ID: 3})
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/appointments/3/availability", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleSearchAppointmentAvailability_InvalidDate(t *testing.T) {
	q := minimalSearchQuerier(db.Experiment{ID: 5, LabID: 1}, db.Appointment{ID: 3})
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/appointments/3/availability?start_date=not-a-date&end_date=2026-09-01", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleSearchAppointmentAvailability_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetAppointmentLabIDFunc: func(ctx context.Context, id int64) (int64, error) {
			return 0, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/appointments/404/availability?start_date=2026-09-01&end_date=2026-09-01", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleScheduleAppointment_SlotNoLongerAvailable(t *testing.T) {
	experiment := db.Experiment{ID: 5, LabID: 1, DurationMinutes: 30}
	appointment := db.Appointment{ID: 3, ExperimentID: 5, SiblingComing: "unknown"}
	q := minimalSearchQuerier(experiment, appointment)
	s, cookie := newAuthenticatedTestServer(q, 7)

	// The whole day is open (minimalSearchQuerier has no constraints), but
	// 25:00 isn't a real time of day the grid covers -- stringToClockTime
	// would reject that anyway, so use a request no valid grid slot could
	// ever match instead: a time before the grid's start (06:00).
	rec := doRequest(t, s, http.MethodPost, "/appointments/3/schedule", cookie, scheduleAppointmentRequest{
		Date: "2026-09-01", StartTime: "02:00",
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusConflict, rec.Body)
	}
}

func TestHandleScheduleAppointment_InvalidDate(t *testing.T) {
	q := &dbfake.Querier{
		GetAppointmentLabIDFunc: func(ctx context.Context, id int64) (int64, error) {
			return 1, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/appointments/3/schedule", cookie, scheduleAppointmentRequest{
		Date: "not-a-date", StartTime: "09:00",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleScheduleAppointment_Success(t *testing.T) {
	experiment := db.Experiment{ID: 5, LabID: 1, DurationMinutes: 30}
	appointment := db.Appointment{ID: 3, ExperimentID: 5, SiblingComing: "unknown"}
	q := minimalSearchQuerier(experiment, appointment)

	var scheduleArgs db.ScheduleAppointmentParams
	q.ScheduleAppointmentFunc = func(ctx context.Context, arg db.ScheduleAppointmentParams) (db.Appointment, error) {
		scheduleArgs = arg
		return db.Appointment{
			ID: arg.ID, ExperimentID: 5, Status: "pending",
			ScheduleDate: arg.ScheduleDate, ScheduleTimeStart: arg.ScheduleTimeStart, ScheduleTimeEnd: arg.ScheduleTimeEnd,
		}, nil
	}
	q.CreateAuditEventFunc = func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
		return db.AuditEvent{ID: 1}, nil
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/appointments/3/schedule", cookie, scheduleAppointmentRequest{
		Date: "2026-09-01", StartTime: "09:00",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if scheduleArgs.ID != 3 {
		t.Errorf("ScheduleAppointment called for ID=%d, want 3", scheduleArgs.ID)
	}
	if !scheduleArgs.ScheduleDate.Time.Equal(pgtype.Date{}.Time) && scheduleArgs.ScheduleDate.Time.Format(dateLayout) != "2026-09-01" {
		t.Errorf("ScheduleDate = %v, want 2026-09-01", scheduleArgs.ScheduleDate.Time)
	}
	got := decodeBody[appointmentResponse](t, rec)
	if got.Status != "pending" {
		t.Errorf("response Status = %q, want \"pending\"", got.Status)
	}
}

func TestHandleReleaseAppointment_Success(t *testing.T) {
	q := &dbfake.Querier{
		GetAppointmentLabIDFunc: func(ctx context.Context, id int64) (int64, error) {
			return 1, nil
		},
		ReleaseAppointmentFunc: func(ctx context.Context, id int64) (db.Appointment, error) {
			return db.Appointment{ID: id, Status: "released"}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/appointments/3/release", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[appointmentResponse](t, rec)
	if got.Status != "released" {
		t.Errorf("response Status = %q, want \"released\"", got.Status)
	}
}

// TestHandleReleaseAppointment_NotFound covers releasing an appointment
// that's already released (or scheduled/completed outside the
// releasable statuses): ReleaseAppointment's WHERE clause matches no
// row, so pgx.ErrNoRows becomes a 404 via writeDBError.
func TestHandleReleaseAppointment_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetAppointmentLabIDFunc: func(ctx context.Context, id int64) (int64, error) {
			return 1, nil
		},
		ReleaseAppointmentFunc: func(ctx context.Context, id int64) (db.Appointment, error) {
			return db.Appointment{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/appointments/3/release", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleReleaseAppointment_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		GetAppointmentLabIDFunc: func(ctx context.Context, id int64) (int64, error) {
			return 1, nil
		},
		ReleaseAppointmentFunc: func(ctx context.Context, id int64) (db.Appointment, error) {
			return db.Appointment{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/appointments/3/release", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleListAppointmentsByExperiment_Success(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		ListAppointmentsByExperimentFunc: func(ctx context.Context, arg db.ListAppointmentsByExperimentParams) ([]db.Appointment, error) {
			return []db.Appointment{{ID: 1, ExperimentID: arg.ExperimentID, Status: "to_be_scheduled"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/5/appointments?status=to_be_scheduled", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]appointmentResponse](t, rec)
	if len(got) != 1 || got[0].ExperimentID != 5 {
		t.Errorf("response = %+v", got)
	}
}

func TestHandleListAppointmentsByExperiment_InvalidExperimentID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/not-a-number/appointments", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleListAppointmentsByExperiment_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		ListAppointmentsByExperimentFunc: func(ctx context.Context, arg db.ListAppointmentsByExperimentParams) ([]db.Appointment, error) {
			return nil, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/5/appointments", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
