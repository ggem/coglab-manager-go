package reminders

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/mail"
)

const staffDigestJobName = "staff_digest"

// These must match internal/httpapi's ActionAppointmentScheduled/
// ActionAppointmentReleased constants -- duplicated here as plain string
// literals rather than imported, since a domain package depending on the
// HTTP transport layer for two constant strings would invert this
// codebase's usual dependency direction. appointment.arrived is
// deliberately excluded: arriving doesn't change what's still upcoming
// for a recipient.
var staffDigestTriggerActions = []string{
	"appointment.scheduled",
	"appointment.released",
}

// RunStaffDigest emails every staff member with at least one
// schedule-affecting change since the digest's last run their current
// upcoming Pending schedule in the affected lab -- ported from legacy's
// change_status-driven daily email, using audit_events as the change
// signal instead of a denormalized dirty-flag column.
//
// A Send failure for one recipient is logged and the pass continues to
// the next recipient rather than aborting the batch; that recipient
// simply isn't retried next run either, since the cursor still advances
// at the end of a pass that reached the send loop at all (a query
// failure before that point leaves the cursor untouched, so the next
// tick retries the same window). This is "at most once" delivery, not
// "at least once" -- building real per-recipient retry/dead-letter
// handling is more machinery than this replaces in legacy, which had no
// error handling at all in the equivalent path.
func RunStaffDigest(ctx context.Context, queries db.Querier, mailer mail.Sender, logger *slog.Logger, now time.Time) error {
	lastRun, err := queries.GetJobLastRun(ctx, staffDigestJobName)
	if errors.Is(err, pgx.ErrNoRows) {
		// First run ever: there's no prior cursor to compare
		// audit_events.occurred_at against. Initialize the cursor to
		// Postgres's own now() (via UpsertJobLastRun, which always
		// stamps server-side -- see its query comment) and skip
		// notifying on this pass, rather than catching up on every
		// historical schedule-affecting change ever recorded.
		return queries.UpsertJobLastRun(ctx, staffDigestJobName)
	}
	if err != nil {
		return fmt.Errorf("get digest last run: %w", err)
	}

	appointmentIDs, err := queries.ListChangedAppointmentIDsSince(ctx, db.ListChangedAppointmentIDsSinceParams{
		Actions: staffDigestTriggerActions,
		Since:   lastRun,
	})
	if err != nil {
		return fmt.Errorf("list changed appointments: %w", err)
	}

	if len(appointmentIDs) > 0 {
		recipients, err := queries.ListRecipientsForAppointments(ctx, appointmentIDs)
		if err != nil {
			return fmt.Errorf("list digest recipients: %w", err)
		}

		today := pgtype.Date{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), Valid: true}
		for _, recipient := range recipients {
			pending, err := queries.ListPendingAppointmentsForUserInLab(ctx, db.ListPendingAppointmentsForUserInLabParams{
				LabID: recipient.LabID, Today: today, UserID: recipient.UserID,
			})
			if err != nil {
				logger.Error("list pending appointments for digest recipient", "user_id", recipient.UserID, "lab_id", recipient.LabID, "error", err)
				continue
			}
			if len(pending) == 0 {
				continue
			}

			msg := mail.Message{
				To:      recipient.Email,
				Subject: fmt.Sprintf("Your %s Lab Schedule", recipient.LabShortName),
				Body:    digestBody(pending),
			}
			if err := mailer.Send(ctx, msg); err != nil {
				logger.Error("send staff digest", "user_id", recipient.UserID, "lab_id", recipient.LabID, "error", err)
			}
		}
	}

	if err := queries.UpsertJobLastRun(ctx, staffDigestJobName); err != nil {
		return fmt.Errorf("advance digest cursor: %w", err)
	}
	return nil
}

func digestBody(pending []db.ListPendingAppointmentsForUserInLabRow) string {
	var b strings.Builder
	for _, p := range pending {
		fmt.Fprintf(&b, "%s %s: %s (%s %s) (%s)\n",
			p.ScheduleDate.Time.Format("2006-01-02"), clockTime(p.ScheduleTimeStart),
			p.ExperimentName, p.ChildFirstName, p.ChildLastName, p.RoleNames)
	}
	return b.String()
}

// clockTime formats a pgtype.Time (a duration since midnight) as
// "HH:MM". A small local duplicate of internal/httpapi's pgconv.go
// helpers of the same shape -- not imported from there, since a domain
// package depending on the HTTP transport layer for wire-formatting
// helpers would invert this codebase's usual dependency direction.
func clockTime(t pgtype.Time) string {
	d := time.Duration(t.Microseconds) * time.Microsecond
	return fmt.Sprintf("%02d:%02d", d/time.Hour, (d%time.Hour)/time.Minute)
}
