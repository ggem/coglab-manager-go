package reminders

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/mail"
)

// familyReminderScanInterval is how often the reminder loop checks for
// newly-due appointments -- an operational polling interval, not a
// product decision like the reminder lead time is, so it's a plain
// constant rather than configurable.
const familyReminderScanInterval = 30 * time.Minute

// Scheduler runs RunStaffDigest and RunFamilyReminders on a real clock,
// replacing legacy's external cron+curl with two in-process goroutines.
// It has no logic of its own beyond timing -- see digest.go/
// family_reminder.go for what each pass actually does.
type Scheduler struct {
	queries    db.Querier
	mailer     mail.Sender
	logger     *slog.Logger
	leadTime   time.Duration
	digestHour int

	// Overridable by tests in this package (unexported: no public option
	// needed) to replace wall-clock waits with fast, fixed-interval
	// ticking. Zero value means "use the real defaults."
	digestInterval   time.Duration
	reminderInterval time.Duration
	now              func() time.Time

	wg sync.WaitGroup
}

// NewScheduler builds a Scheduler that emails staff a digest daily at
// digestHour (0-23, in the server process's local time) and scans for
// due family reminders every 30 minutes, reminding a family leadTime
// before their appointment.
func NewScheduler(queries db.Querier, mailer mail.Sender, logger *slog.Logger, leadTime time.Duration, digestHour int) *Scheduler {
	return &Scheduler{
		queries: queries, mailer: mailer, logger: logger,
		leadTime: leadTime, digestHour: digestHour,
	}
}

// Run starts both scheduled loops in their own goroutines; they stop
// when ctx is canceled. Call Wait afterward to block until they've
// actually stopped, e.g. during graceful shutdown.
func (s *Scheduler) Run(ctx context.Context) {
	s.wg.Add(2)
	go func() {
		defer s.wg.Done()
		s.runDigestLoop(ctx)
	}()
	go func() {
		defer s.wg.Done()
		s.runReminderLoop(ctx)
	}()
}

// Wait blocks until both loops started by Run have stopped.
func (s *Scheduler) Wait() {
	s.wg.Wait()
}

func (s *Scheduler) runDigestLoop(ctx context.Context) {
	if s.digestInterval > 0 {
		// Test override: skip wall-clock alignment, just tick at the
		// given (fast) interval.
		ticker := time.NewTicker(s.digestInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runDigestPass(ctx)
			}
		}
	}

	timer := time.NewTimer(time.Until(nextHour(s.nowOrDefault(), s.digestHour)))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.runDigestPass(ctx)
			timer.Reset(24 * time.Hour)
		}
	}
}

func (s *Scheduler) runReminderLoop(ctx context.Context) {
	interval := s.reminderInterval
	if interval <= 0 {
		interval = familyReminderScanInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := RunFamilyReminders(ctx, s.queries, s.mailer, s.logger, s.nowOrDefault(), s.leadTime); err != nil {
				s.logger.Error("run family reminders", "error", err)
			}
		}
	}
}

func (s *Scheduler) runDigestPass(ctx context.Context) {
	if err := RunStaffDigest(ctx, s.queries, s.mailer, s.logger, s.nowOrDefault()); err != nil {
		s.logger.Error("run staff digest", "error", err)
	}
}

func (s *Scheduler) nowOrDefault() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// nextHour returns the next occurrence of hour:00 at or after now, in
// now's own location -- computing a wall-clock-aligned initial delay is
// the one piece of real scheduling logic here; everything after the
// first fire is just a plain 24h ticker.
func nextHour(now time.Time, hour int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
