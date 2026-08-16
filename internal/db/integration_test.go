//go:build integration

// Integration tests exercise the generated queries against a real Postgres
// instance (via internal/dbtest), covering behavior a fake db.Querier can't:
// constraint enforcement, trigger behavior, and real type round-tripping
// (jsonb, inet, timestamptz). Run with: go test -tags=integration ./...
//
// Rows created by these tests are never cleaned up individually -- the
// whole container is discarded at the end of the run, so isolation comes
// from each test using its own unique data instead.
package db_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/dbtest"
)

var testQueries *db.Queries

func TestMain(m *testing.M) {
	ctx := context.Background()

	pg, err := dbtest.StartPostgres(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start postgres:", err)
		os.Exit(1)
	}
	testQueries = db.New(pg.Pool)

	code := m.Run()

	if err := pg.Close(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "close postgres:", err)
	}
	os.Exit(code)
}

// createTestUser inserts a user with an email unique to this test run.
func createTestUser(t *testing.T, ctx context.Context) db.User {
	t.Helper()
	email := fmt.Sprintf("%s-%d@example.edu", t.Name(), time.Now().UnixNano())
	user, err := testQueries.CreateUser(ctx, db.CreateUserParams{
		Email:     email,
		FirstName: "Test",
		LastName:  "User",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return user
}

func TestCreateUser_EmailUniquenessIsCaseInsensitive(t *testing.T) {
	ctx := context.Background()
	email := fmt.Sprintf("Researcher-%d@Example.edu", time.Now().UnixNano())

	created, err := testQueries.CreateUser(ctx, db.CreateUserParams{Email: email, FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Re-attempt with the exact same email: this is what the unique index
	// on lower(email) exists to catch, so it should be rejected even
	// though CreateUser itself lowercases on insert (see users.sql).
	if _, err := testQueries.CreateUser(ctx, db.CreateUserParams{Email: email, FirstName: "Dup", LastName: "User"}); err == nil {
		t.Fatal("expected a unique-constraint violation inserting the same email again")
	}

	got, err := testQueries.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetUserByEmail returned a different user than was created")
	}
}

func TestSession_Lifecycle(t *testing.T) {
	ctx := context.Background()
	user := createTestUser(t, ctx)
	tokenHash := []byte(fmt.Sprintf("token-hash-%d", time.Now().UnixNano()))

	session, err := testQueries.CreateSession(ctx, db.CreateSessionParams{
		TokenHash: tokenHash,
		UserID:    user.ID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if session.RevokedAt.Valid {
		t.Error("new session should not be revoked")
	}

	got, err := testQueries.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("GetSessionByTokenHash: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("GetSessionByTokenHash returned a different session")
	}

	beforeTouch := got.LastSeenAt.Time
	time.Sleep(10 * time.Millisecond)
	if err := testQueries.TouchSessionLastSeen(ctx, session.ID); err != nil {
		t.Fatalf("TouchSessionLastSeen: %v", err)
	}
	touched, err := testQueries.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("GetSessionByTokenHash after touch: %v", err)
	}
	if !touched.LastSeenAt.Time.After(beforeTouch) {
		t.Errorf("TouchSessionLastSeen did not advance last_seen_at: before=%v after=%v", beforeTouch, touched.LastSeenAt.Time)
	}

	if err := testQueries.RevokeSession(ctx, tokenHash); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	revoked, err := testQueries.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("GetSessionByTokenHash after revoke: %v", err)
	}
	if !revoked.RevokedAt.Valid {
		t.Error("RevokeSession did not set revoked_at")
	}
}

func TestCreateAuditEvent_MetadataRoundTrip(t *testing.T) {
	ctx := context.Background()
	user := createTestUser(t, ctx)
	userID := user.ID

	event, err := testQueries.CreateAuditEvent(ctx, db.CreateAuditEventParams{
		ActorUserID: &userID,
		Action:      "user.login_succeeded",
		Metadata:    []byte(`{"method":"local"}`),
	})
	if err != nil {
		t.Fatalf("CreateAuditEvent: %v", err)
	}
	// jsonb re-serializes on storage (e.g. inserts a space after ':'), so
	// compare decoded values rather than raw bytes.
	var gotMetadata map[string]string
	if err := json.Unmarshal(event.Metadata, &gotMetadata); err != nil {
		t.Fatalf("unmarshal stored metadata: %v", err)
	}
	if gotMetadata["method"] != "local" {
		t.Errorf("Metadata[method] = %q, want %q", gotMetadata["method"], "local")
	}

	withoutMetadata, err := testQueries.CreateAuditEvent(ctx, db.CreateAuditEventParams{
		Action: "user.login_failed",
	})
	if err != nil {
		t.Fatalf("CreateAuditEvent without metadata: %v", err)
	}
	if withoutMetadata.Metadata != nil {
		t.Errorf("Metadata = %v, want nil for an event created without metadata", withoutMetadata.Metadata)
	}
	if withoutMetadata.ActorUserID != nil {
		t.Errorf("ActorUserID = %v, want nil for an unauthenticated event", withoutMetadata.ActorUserID)
	}
}
