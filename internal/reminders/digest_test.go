package reminders

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
	"github.com/ggem/coglab-manager-go/internal/mail/mailfake"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunStaffDigest_NoChanges_NoMailButCursorAdvances(t *testing.T) {
	var upserted bool
	q := &dbfake.Querier{
		GetJobLastRunFunc: func(ctx context.Context, jobName string) (pgtype.Timestamptz, error) {
			return pgtype.Timestamptz{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}, nil
		},
		ListChangedAppointmentIDsSinceFunc: func(ctx context.Context, arg db.ListChangedAppointmentIDsSinceParams) ([]int64, error) {
			return nil, nil
		},
		UpsertJobLastRunFunc: func(ctx context.Context, jobName string) error {
			upserted = true
			return nil
		},
	}
	sender := &mailfake.Sender{}
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

	if err := RunStaffDigest(context.Background(), q, sender, discardLogger(), now); err != nil {
		t.Fatalf("RunStaffDigest: %v", err)
	}
	if len(sender.Messages()) != 0 {
		t.Errorf("Messages = %+v, want none", sender.Messages())
	}
	if !upserted {
		t.Error("cursor was not advanced")
	}
}

// TestRunStaffDigest_FirstEverRun_NoCatchUp confirms a job with no
// stored cursor yet initializes one (via UpsertJobLastRun, which always
// stamps Postgres's own now()) and returns without even querying for
// changed appointments -- there's no meaningful prior boundary to query
// against yet, so a fresh deployment starts observing from here forward
// rather than catching up on every historical change ever recorded.
func TestRunStaffDigest_FirstEverRun_NoCatchUp(t *testing.T) {
	var upserted bool
	q := &dbfake.Querier{
		GetJobLastRunFunc: func(ctx context.Context, jobName string) (pgtype.Timestamptz, error) {
			return pgtype.Timestamptz{}, pgx.ErrNoRows
		},
		// ListChangedAppointmentIDsSinceFunc deliberately unset: calling
		// it would panic, proving the first-ever run doesn't query at
		// all.
		UpsertJobLastRunFunc: func(ctx context.Context, jobName string) error {
			upserted = true
			return nil
		},
	}

	if err := RunStaffDigest(context.Background(), q, &mailfake.Sender{}, discardLogger(), time.Now()); err != nil {
		t.Fatalf("RunStaffDigest: %v", err)
	}
	if !upserted {
		t.Error("cursor was not initialized on first-ever run")
	}
}

func TestRunStaffDigest_ChangedAppointment_SendsExpectedMail(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	q := &dbfake.Querier{
		GetJobLastRunFunc: func(ctx context.Context, jobName string) (pgtype.Timestamptz, error) {
			return pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true}, nil
		},
		ListChangedAppointmentIDsSinceFunc: func(ctx context.Context, arg db.ListChangedAppointmentIDsSinceParams) ([]int64, error) {
			return []int64{42}, nil
		},
		ListRecipientsForAppointmentsFunc: func(ctx context.Context, appointmentIDs []int64) ([]db.ListRecipientsForAppointmentsRow, error) {
			if len(appointmentIDs) != 1 || appointmentIDs[0] != 42 {
				t.Errorf("ListRecipientsForAppointments called with %v, want [42]", appointmentIDs)
			}
			return []db.ListRecipientsForAppointmentsRow{
				{UserID: 7, Email: "staff@example.edu", FirstName: "Staff", LastName: "Member", LabID: 1, LabShortName: "CDC"},
			}, nil
		},
		ListPendingAppointmentsForUserInLabFunc: func(ctx context.Context, arg db.ListPendingAppointmentsForUserInLabParams) ([]db.ListPendingAppointmentsForUserInLabRow, error) {
			if arg.UserID != 7 || arg.LabID != 1 {
				t.Errorf("ListPendingAppointmentsForUserInLab called with %+v", arg)
			}
			return []db.ListPendingAppointmentsForUserInLabRow{
				{
					AppointmentID:     42,
					ScheduleDate:      pgtype.Date{Time: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), Valid: true},
					ScheduleTimeStart: pgtype.Time{Microseconds: int64(9 * time.Hour / time.Microsecond), Valid: true},
					ExperimentName:    "Looking Study",
					ChildFirstName:    "Kid",
					ChildLastName:     "Test",
					RoleNames:         "Experimenter",
				},
			}, nil
		},
		UpsertJobLastRunFunc: func(ctx context.Context, jobName string) error {
			return nil
		},
	}
	sender := &mailfake.Sender{}

	if err := RunStaffDigest(context.Background(), q, sender, discardLogger(), now); err != nil {
		t.Fatalf("RunStaffDigest: %v", err)
	}

	got := sender.Messages()
	if len(got) != 1 {
		t.Fatalf("Messages = %+v, want exactly one", got)
	}
	if got[0].To != "staff@example.edu" {
		t.Errorf("To = %q, want staff@example.edu", got[0].To)
	}
	if got[0].Subject != "Your CDC Lab Schedule" {
		t.Errorf("Subject = %q, want %q", got[0].Subject, "Your CDC Lab Schedule")
	}
	wantLine := "2026-01-05 09:00: Looking Study (Kid Test) (Experimenter)"
	if !strings.Contains(got[0].Body, wantLine) {
		t.Errorf("Body = %q, want a line %q", got[0].Body, wantLine)
	}
}

func TestRunStaffDigest_RecipientWithNoCurrentPending_NoMail(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	q := &dbfake.Querier{
		GetJobLastRunFunc: func(ctx context.Context, jobName string) (pgtype.Timestamptz, error) {
			return pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true}, nil
		},
		ListChangedAppointmentIDsSinceFunc: func(ctx context.Context, arg db.ListChangedAppointmentIDsSinceParams) ([]int64, error) {
			return []int64{42}, nil
		},
		ListRecipientsForAppointmentsFunc: func(ctx context.Context, appointmentIDs []int64) ([]db.ListRecipientsForAppointmentsRow, error) {
			return []db.ListRecipientsForAppointmentsRow{
				{UserID: 7, Email: "staff@example.edu", LabID: 1, LabShortName: "CDC"},
			}, nil
		},
		// The appointment that changed was released, so this recipient
		// now has nothing currently pending -- they shouldn't get an
		// empty digest email.
		ListPendingAppointmentsForUserInLabFunc: func(ctx context.Context, arg db.ListPendingAppointmentsForUserInLabParams) ([]db.ListPendingAppointmentsForUserInLabRow, error) {
			return nil, nil
		},
		UpsertJobLastRunFunc: func(ctx context.Context, jobName string) error {
			return nil
		},
	}
	sender := &mailfake.Sender{}

	if err := RunStaffDigest(context.Background(), q, sender, discardLogger(), now); err != nil {
		t.Fatalf("RunStaffDigest: %v", err)
	}
	if len(sender.Messages()) != 0 {
		t.Errorf("Messages = %+v, want none", sender.Messages())
	}
}

func TestRunStaffDigest_SendFailureForOneRecipient_ContinuesAndAdvancesCursor(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	var upserted bool
	q := &dbfake.Querier{
		GetJobLastRunFunc: func(ctx context.Context, jobName string) (pgtype.Timestamptz, error) {
			return pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true}, nil
		},
		ListChangedAppointmentIDsSinceFunc: func(ctx context.Context, arg db.ListChangedAppointmentIDsSinceParams) ([]int64, error) {
			return []int64{42}, nil
		},
		ListRecipientsForAppointmentsFunc: func(ctx context.Context, appointmentIDs []int64) ([]db.ListRecipientsForAppointmentsRow, error) {
			return []db.ListRecipientsForAppointmentsRow{
				{UserID: 7, Email: "staff@example.edu", LabID: 1, LabShortName: "CDC"},
			}, nil
		},
		ListPendingAppointmentsForUserInLabFunc: func(ctx context.Context, arg db.ListPendingAppointmentsForUserInLabParams) ([]db.ListPendingAppointmentsForUserInLabRow, error) {
			return []db.ListPendingAppointmentsForUserInLabRow{{AppointmentID: 42, ScheduleDate: pgtype.Date{Time: now, Valid: true}}}, nil
		},
		UpsertJobLastRunFunc: func(ctx context.Context, jobName string) error {
			upserted = true
			return nil
		},
	}
	sender := &mailfake.Sender{Err: errors.New("smtp relay down")}

	if err := RunStaffDigest(context.Background(), q, sender, discardLogger(), now); err != nil {
		t.Fatalf("RunStaffDigest: %v (a per-recipient send failure should not fail the pass)", err)
	}
	if !upserted {
		t.Error("cursor was not advanced despite reaching the end of the pass")
	}
}

func TestRunStaffDigest_QueryFailure_CursorNotAdvanced(t *testing.T) {
	q := &dbfake.Querier{
		GetJobLastRunFunc: func(ctx context.Context, jobName string) (pgtype.Timestamptz, error) {
			return pgtype.Timestamptz{Time: time.Now(), Valid: true}, nil
		},
		ListChangedAppointmentIDsSinceFunc: func(ctx context.Context, arg db.ListChangedAppointmentIDsSinceParams) ([]int64, error) {
			return nil, errors.New("connection reset by peer")
		},
		// UpsertJobLastRunFunc deliberately unset: a call to it panics,
		// proving the cursor is never touched on a query failure.
	}

	err := RunStaffDigest(context.Background(), q, &mailfake.Sender{}, discardLogger(), time.Now())

	if err == nil {
		t.Fatal("RunStaffDigest returned nil error, want the query failure surfaced")
	}
}
