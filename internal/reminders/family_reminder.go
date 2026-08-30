package reminders

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/mail"
)

// RunFamilyReminders emails a family an informational reminder for any
// scheduled appointment starting within leadTime that hasn't been
// reminded yet -- a new feature, not a legacy port (legacy's only
// family-facing "reminder" was an undated manual staff phone call). No
// confirm/decline action: just the appointment's date, time, and study
// name.
//
// Idempotency is per-appointment (reminder_sent_at), not a shared
// cursor like the staff digest: each appointment's own flag is the
// source of truth for "have I reminded this family yet," so there's
// nothing to lose track of across runs the way a global cursor could.
// A Send failure is logged and the pass continues to the next
// appointment, same "at most once" reasoning as RunStaffDigest.
func RunFamilyReminders(ctx context.Context, queries db.Querier, mailer mail.Sender, logger *slog.Logger, now time.Time, leadTime time.Duration) error {
	due, err := queries.ListAppointmentsDueForReminder(ctx, pgtype.Timestamp{Time: now.Add(leadTime), Valid: true})
	if err != nil {
		return fmt.Errorf("list appointments due for reminder: %w", err)
	}

	for _, appt := range due {
		if appt.GuardianEmail == "" {
			logger.Warn("skipping family reminder: no guardian email on file", "appointment_id", appt.AppointmentID)
			continue
		}

		msg := mail.Message{
			To:      appt.GuardianEmail,
			Subject: fmt.Sprintf("Reminder: %s appointment on %s", appt.ExperimentName, appt.ScheduleDate.Time.Format("2006-01-02")),
			Body: fmt.Sprintf(
				"Hi %s,\n\nThis is a reminder of %s's upcoming appointment:\n\n%s at %s: %s\n",
				appt.GuardianFirstName, appt.ChildFirstName,
				appt.ScheduleDate.Time.Format("2006-01-02"), clockTime(appt.ScheduleTimeStart), appt.ExperimentName,
			),
		}
		if err := mailer.Send(ctx, msg); err != nil {
			logger.Error("send family reminder", "appointment_id", appt.AppointmentID, "error", err)
			continue
		}

		if err := queries.MarkAppointmentReminderSent(ctx, appt.AppointmentID); err != nil {
			logger.Error("mark appointment reminder sent", "appointment_id", appt.AppointmentID, "error", err)
		}
	}
	return nil
}
