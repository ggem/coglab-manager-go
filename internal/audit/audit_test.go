package audit

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestRecorder_Record(t *testing.T) {
	var captured db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			captured = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	r := NewRecorder(q)

	actorID := int64(7)
	labID := int64(3)
	entityType := "user"
	entityID := int64(99)

	err := r.Record(context.Background(), Event{
		ActorUserID: &actorID,
		LabID:       &labID,
		Action:      "user.login_succeeded",
		EntityType:  &entityType,
		EntityID:    &entityID,
		Metadata:    map[string]string{"method": "local"},
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	if captured.Action != "user.login_succeeded" {
		t.Errorf("Action = %q, want %q", captured.Action, "user.login_succeeded")
	}
	if captured.ActorUserID == nil || *captured.ActorUserID != actorID {
		t.Errorf("ActorUserID = %v, want %d", captured.ActorUserID, actorID)
	}
	if captured.LabID == nil || *captured.LabID != labID {
		t.Errorf("LabID = %v, want %d", captured.LabID, labID)
	}
	if captured.EntityType == nil || *captured.EntityType != entityType {
		t.Errorf("EntityType = %v, want %q", captured.EntityType, entityType)
	}
	if captured.EntityID == nil || *captured.EntityID != entityID {
		t.Errorf("EntityID = %v, want %d", captured.EntityID, entityID)
	}

	var gotMetadata map[string]string
	if err := json.Unmarshal(captured.Metadata, &gotMetadata); err != nil {
		t.Fatalf("unmarshal captured metadata: %v", err)
	}
	if gotMetadata["method"] != "local" {
		t.Errorf("metadata[method] = %q, want %q", gotMetadata["method"], "local")
	}
}

func TestRecorder_Record_NilMetadataStoredAsNull(t *testing.T) {
	var captured db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			captured = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	r := NewRecorder(q)

	err := r.Record(context.Background(), Event{Action: "user.login_failed"})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	if captured.Metadata != nil {
		t.Errorf("Metadata = %v, want nil", captured.Metadata)
	}
	if captured.ActorUserID != nil {
		t.Errorf("ActorUserID = %v, want nil (unauthenticated event)", captured.ActorUserID)
	}
}

func TestRecorder_Record_QueryError(t *testing.T) {
	q := &dbfake.Querier{
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{}, context.DeadlineExceeded
		},
	}
	r := NewRecorder(q)

	if err := r.Record(context.Background(), Event{Action: "user.login_succeeded"}); err == nil {
		t.Error("Record() error = nil, want non-nil when the underlying query fails")
	}
}
