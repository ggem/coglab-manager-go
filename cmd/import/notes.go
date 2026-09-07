package main

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// legacyImportUserEmail is the pseudo-user every imported freeform note
// is attributed to, so an imported note never looks like it came from a
// real staff member who didn't actually write it (see the M10 planning
// memory). Created once at the start of a real import run; its id is
// threaded through to every note-producing importer below.
const legacyImportUserEmail = "legacy-import@coglab.internal"

func importLegacyImportUser(ctx context.Context, tx pgx.Tx) (int64, error) {
	// Deliberately far outside any real legacy member_id range so this
	// pseudo-user's id can never collide with a real imported user.
	const reservedID = int64(900000000)
	err := insertRow(ctx, tx, "users", map[string]any{
		"id":         reservedID,
		"email":      legacyImportUserEmail,
		"first_name": "Legacy",
		"last_name":  "Import",
	})
	if err != nil {
		return 0, err
	}
	return reservedID, nil
}

// addNote inserts one notes row if body is non-empty (legacy leaves
// most of these fields "" rather than NULL, and an empty note isn't
// worth importing).
func addNote(ctx context.Context, tx pgx.Tx, entityType string, entityID, authorUserID int64, body string, createdAt *time.Time) error {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	fields := map[string]any{
		"entity_type":    entityType,
		"entity_id":      entityID,
		"author_user_id": authorUserID,
		"body":           body,
	}
	if createdAt != nil {
		fields["created_at"] = *createdAt
	}
	return insertAssoc(ctx, tx, "notes", fields)
}

// callLogEntry is one parsed "<timestamp>: <text>" entry from legacy's
// calling_log/changes_log-style freeform log fields.
type callLogEntry struct {
	at   time.Time
	body string
}

var logEntryAnchor = regexp.MustCompile(`(?m)^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}): `)

// parseTimestampedLog splits legacy's "concat(log, now(), ': ', text,
// '\n')" convention (confirmed from org.ggem.kids.participants.ss's
// appointments:schedule:update-call-log, which builds calling_log this
// exact way) into individual timestamped entries. An entry's body runs
// until the next recognized timestamp anchor, not just to the next
// newline, since the original free-text (typed by a staff member) can
// itself contain embedded newlines. ok is false when nothing in raw
// matches the expected format at all -- the caller falls back to
// importing the whole blob as one plain note rather than dropping it.
func parseTimestampedLog(raw string) (entries []callLogEntry, ok bool) {
	locs := logEntryAnchor.FindAllStringSubmatchIndex(raw, -1)
	if len(locs) == 0 {
		return nil, false
	}
	for i, loc := range locs {
		tsStart, tsEnd := loc[2], loc[3]
		bodyStart := loc[1]
		bodyEnd := len(raw)
		if i+1 < len(locs) {
			bodyEnd = locs[i+1][0]
		}
		ts, err := time.Parse("2006-01-02 15:04:05", raw[tsStart:tsEnd])
		if err != nil {
			continue
		}
		body := strings.TrimRight(raw[bodyStart:bodyEnd], "\n")
		if body == "" {
			continue
		}
		entries = append(entries, callLogEntry{at: ts, body: body})
	}
	// A timestamp anchor matching isn't enough on its own -- every
	// matched timestamp could still fail to parse (e.g. a malformed
	// "2026-99-99") or have an empty body, in which case entries stays
	// empty and the raw text must not be silently dropped: the caller's
	// documented fallback (one plain note with the original text) only
	// runs when ok is false, so ok must reflect whether anything was
	// actually recovered, not just whether the format looked promising.
	return entries, len(entries) > 0
}

// importCallingLog parses notes_appointments.calling_log into one notes
// row per call attempt (each keeping its own original timestamp),
// falling back to a single note with the raw text when the format
// isn't recognized -- per the M10 planning decision to attempt
// structured parsing rather than either dropping this history or
// always flattening it into one blob.
func importCallingLog(ctx context.Context, tx pgx.Tx, appointmentID, authorUserID int64, raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	entries, ok := parseTimestampedLog(raw)
	if !ok {
		return addNote(ctx, tx, "appointment", appointmentID, authorUserID, raw, nil)
	}
	for _, e := range entries {
		at := e.at
		if err := addNote(ctx, tx, "appointment", appointmentID, authorUserID, e.body, &at); err != nil {
			return err
		}
	}
	return nil
}

// importFreeformNotes imports the genuine freeform note fields --
// notes_children.notes/.bc_notes and notes_appointments.notes_appointment
// -- plus calling_log via importCallingLog. Every other notes_* table's
// changes_log/pchanges_log field is intentionally skipped: legacy's own
// mechanical modification log, fully superseded by this app's real
// audit_events (see the M10 planning memory).
func importFreeformNotes(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, authorUserID int64) error {
	childRows, err := queryLegacy(ctx, legacyDB, "select * from notes_children")
	if err != nil {
		return err
	}
	for _, row := range childRows {
		childID := row.int64("child_id")
		err := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			if err := addNote(ctx, sp, "child", childID, authorUserID, row.str("notes"), nil); err != nil {
				return err
			}
			return addNote(ctx, sp, "child", childID, authorUserID, row.str("bc_notes"), nil)
		})
		if err != nil {
			report.Error("notes(child)", childID, err)
			continue
		}
		report.Ok("notes(child)")
	}

	apptRows, err := queryLegacy(ctx, legacyDB, "select * from notes_appointments")
	if err != nil {
		return err
	}
	for _, row := range apptRows {
		appointmentID := row.int64("appointment_id")
		err := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			if err := addNote(ctx, sp, "appointment", appointmentID, authorUserID, row.str("notes_appointment"), nil); err != nil {
				return err
			}
			return importCallingLog(ctx, sp, appointmentID, authorUserID, row.str("calling_log"))
		})
		if err != nil {
			report.Error("notes(appointment)", appointmentID, err)
			continue
		}
		report.Ok("notes(appointment)")
	}

	return bumpSequence(ctx, tx, "notes")
}
