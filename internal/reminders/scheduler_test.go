package reminders

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
	"github.com/ggem/coglab-manager-go/internal/mail/mailfake"
)

func TestScheduler_RunsBothLoopsAndStopsOnCancel(t *testing.T) {
	var digestRuns, reminderRuns atomic.Int32
	q := &dbfake.Querier{
		GetJobLastRunFunc: func(ctx context.Context, jobName string) (pgtype.Timestamptz, error) {
			return pgtype.Timestamptz{}, pgx.ErrNoRows
		},
		ListChangedAppointmentIDsSinceFunc: func(ctx context.Context, arg db.ListChangedAppointmentIDsSinceParams) ([]int64, error) {
			return nil, nil
		},
		UpsertJobLastRunFunc: func(ctx context.Context, jobName string) error {
			// Called on every digest pass, including the first-ever-run
			// short-circuit (which never reaches
			// ListChangedAppointmentIDsSinceFunc) -- the reliable place
			// to count a pass as having fired.
			digestRuns.Add(1)
			return nil
		},
		ListAppointmentsDueForReminderFunc: func(ctx context.Context, dueBefore pgtype.Timestamp) ([]db.ListAppointmentsDueForReminderRow, error) {
			reminderRuns.Add(1)
			return nil, nil
		},
	}

	s := NewScheduler(q, &mailfake.Sender{}, discardLogger(), 24*time.Hour, 17)
	s.digestInterval = 5 * time.Millisecond
	s.reminderInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	s.Run(ctx)

	deadline := time.After(2 * time.Second)
	for digestRuns.Load() == 0 || reminderRuns.Load() == 0 {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for both loops to fire at least once (digest=%d, reminder=%d)", digestRuns.Load(), reminderRuns.Load())
		case <-time.After(time.Millisecond):
		}
	}

	cancel()

	waited := make(chan struct{})
	go func() {
		s.Wait()
		close(waited)
	}()
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("Scheduler.Wait() did not return within 2s of context cancellation -- a loop leaked")
	}
}

func TestNextHour(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		hour int
		want time.Time
	}{
		{
			name: "later today",
			now:  time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC),
			hour: 17,
			want: time.Date(2026, 1, 1, 17, 0, 0, 0, time.UTC),
		},
		{
			name: "already past today, rolls to tomorrow",
			now:  time.Date(2026, 1, 1, 18, 0, 0, 0, time.UTC),
			hour: 17,
			want: time.Date(2026, 1, 2, 17, 0, 0, 0, time.UTC),
		},
		{
			name: "exactly at the hour rolls to tomorrow",
			now:  time.Date(2026, 1, 1, 17, 0, 0, 0, time.UTC),
			hour: 17,
			want: time.Date(2026, 1, 2, 17, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextHour(tt.now, tt.hour)
			if !got.Equal(tt.want) {
				t.Errorf("nextHour(%v, %d) = %v, want %v", tt.now, tt.hour, got, tt.want)
			}
		})
	}
}
