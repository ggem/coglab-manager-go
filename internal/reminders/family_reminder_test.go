package reminders

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
	"github.com/ggem/coglab-manager-go/internal/mail/mailfake"
)

func TestRunFamilyReminders_NoneDue_NoMail(t *testing.T) {
	q := &dbfake.Querier{
		ListAppointmentsDueForReminderFunc: func(ctx context.Context, dueBefore pgtype.Timestamp) ([]db.ListAppointmentsDueForReminderRow, error) {
			return nil, nil
		},
	}
	sender := &mailfake.Sender{}

	if err := RunFamilyReminders(context.Background(), q, sender, discardLogger(), time.Now(), 24*time.Hour); err != nil {
		t.Fatalf("RunFamilyReminders: %v", err)
	}
	if len(sender.Messages()) != 0 {
		t.Errorf("Messages = %+v, want none", sender.Messages())
	}
}

func TestRunFamilyReminders_DueAppointment_SendsAndMarksSent(t *testing.T) {
	var marked int64
	q := &dbfake.Querier{
		ListAppointmentsDueForReminderFunc: func(ctx context.Context, dueBefore pgtype.Timestamp) ([]db.ListAppointmentsDueForReminderRow, error) {
			return []db.ListAppointmentsDueForReminderRow{
				{
					AppointmentID:     99,
					ScheduleDate:      pgtype.Date{Time: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), Valid: true},
					ScheduleTimeStart: pgtype.Time{Microseconds: int64(9 * time.Hour / time.Microsecond), Valid: true},
					ExperimentName:    "Looking Study",
					ChildFirstName:    "Kid",
					ChildLastName:     "Test",
					GuardianEmail:     "parent@example.edu",
					GuardianFirstName: "Parent",
					GuardianLastName:  "One",
				},
			}, nil
		},
		MarkAppointmentReminderSentFunc: func(ctx context.Context, id int64) error {
			marked = id
			return nil
		},
	}
	sender := &mailfake.Sender{}

	if err := RunFamilyReminders(context.Background(), q, sender, discardLogger(), time.Now(), 24*time.Hour); err != nil {
		t.Fatalf("RunFamilyReminders: %v", err)
	}

	got := sender.Messages()
	if len(got) != 1 {
		t.Fatalf("Messages = %+v, want exactly one", got)
	}
	if got[0].To != "parent@example.edu" {
		t.Errorf("To = %q, want parent@example.edu", got[0].To)
	}
	if !strings.Contains(got[0].Body, "Kid") || !strings.Contains(got[0].Body, "Looking Study") || !strings.Contains(got[0].Body, "09:00") {
		t.Errorf("Body = %q, want it to mention the child, study, and time", got[0].Body)
	}
	if marked != 99 {
		t.Errorf("MarkAppointmentReminderSent called with id=%d, want 99", marked)
	}
}

func TestRunFamilyReminders_NoGuardianEmail_SkipsWithoutMarking(t *testing.T) {
	q := &dbfake.Querier{
		ListAppointmentsDueForReminderFunc: func(ctx context.Context, dueBefore pgtype.Timestamp) ([]db.ListAppointmentsDueForReminderRow, error) {
			return []db.ListAppointmentsDueForReminderRow{
				{AppointmentID: 99, GuardianEmail: ""},
			}, nil
		},
		// MarkAppointmentReminderSentFunc deliberately unset: calling it
		// would panic, proving a skipped appointment isn't marked sent.
	}
	sender := &mailfake.Sender{}

	if err := RunFamilyReminders(context.Background(), q, sender, discardLogger(), time.Now(), 24*time.Hour); err != nil {
		t.Fatalf("RunFamilyReminders: %v", err)
	}
	if len(sender.Messages()) != 0 {
		t.Errorf("Messages = %+v, want none (no guardian email on file)", sender.Messages())
	}
}

func TestRunFamilyReminders_SendFailure_NotMarkedSent(t *testing.T) {
	q := &dbfake.Querier{
		ListAppointmentsDueForReminderFunc: func(ctx context.Context, dueBefore pgtype.Timestamp) ([]db.ListAppointmentsDueForReminderRow, error) {
			return []db.ListAppointmentsDueForReminderRow{
				{AppointmentID: 99, GuardianEmail: "parent@example.edu"},
			}, nil
		},
		// MarkAppointmentReminderSentFunc deliberately unset: calling it
		// would panic, proving a failed send isn't marked sent (so it can
		// be retried on the next scan).
	}
	sender := &mailfake.Sender{Err: errors.New("smtp relay down")}

	if err := RunFamilyReminders(context.Background(), q, sender, discardLogger(), time.Now(), 24*time.Hour); err != nil {
		t.Fatalf("RunFamilyReminders: %v (a per-appointment send failure should not fail the pass)", err)
	}
}

func TestRunFamilyReminders_QueryFailure(t *testing.T) {
	q := &dbfake.Querier{
		ListAppointmentsDueForReminderFunc: func(ctx context.Context, dueBefore pgtype.Timestamp) ([]db.ListAppointmentsDueForReminderRow, error) {
			return nil, errors.New("connection reset by peer")
		},
	}

	err := RunFamilyReminders(context.Background(), q, &mailfake.Sender{}, discardLogger(), time.Now(), 24*time.Hour)

	if err == nil {
		t.Fatal("RunFamilyReminders returned nil error, want the query failure surfaced")
	}
}
