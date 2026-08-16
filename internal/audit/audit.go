// Package audit records who did what, to what, and when. It's an
// append-only trail written explicitly by the code performing an action,
// not inferred later from a debug log or a database trigger -- the legacy
// app's "audit log" was a claim, not a real feature: it was a line written
// to stdout, gated by a mutable debug level an admin could turn off, with
// no durable or queryable storage. This is a real table instead.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/ggem/coglab-manager-go/internal/db"
)

// Event describes something worth recording in the audit trail. The
// packages that produce events (auth, and later the domain packages that
// mutate participants/experiments/appointments) define their own Action
// name constants -- this package only knows how to store an event, not
// what any particular action means.
type Event struct {
	ActorUserID *int64 // nil for unauthenticated events, e.g. a failed login
	LabID       *int64 // nil for platform-level events not scoped to one lab
	Action      string
	EntityType  *string
	EntityID    *int64
	IPAddress   *netip.Addr
	Metadata    any // marshaled to JSON; nil is stored as SQL NULL
}

// Recorder writes audit events.
//
// NewRecorder returns the concrete type rather than an interface, per the
// "accept interfaces, return structs" convention: a caller that needs to
// substitute a fake for its own tests should declare its own narrow
// interface (just a Record method) at the point where it uses this type,
// rather than this package guessing what shape callers will want.
type Recorder struct {
	queries db.Querier
}

func NewRecorder(queries db.Querier) *Recorder {
	return &Recorder{queries: queries}
}

func (r *Recorder) Record(ctx context.Context, event Event) error {
	var metadata []byte
	if event.Metadata != nil {
		m, err := json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("marshal audit metadata: %w", err)
		}
		metadata = m
	}

	_, err := r.queries.CreateAuditEvent(ctx, db.CreateAuditEventParams{
		ActorUserID: event.ActorUserID,
		LabID:       event.LabID,
		Action:      event.Action,
		EntityType:  event.EntityType,
		EntityID:    event.EntityID,
		IpAddress:   event.IPAddress,
		Metadata:    metadata,
	})
	if err != nil {
		return fmt.Errorf("create audit event: %w", err)
	}
	return nil
}
