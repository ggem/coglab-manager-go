package main

import (
	"testing"
	"time"
)

func TestParseTimestampedLog(t *testing.T) {
	raw := "2026-01-15 14:30:00: Left voicemail, will try again tomorrow.\n" +
		"2026-01-14 09:15:00: No answer.\n"
	entries, ok := parseTimestampedLog(raw)
	if !ok {
		t.Fatal("expected ok = true")
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v, want 2", entries)
	}
	want0 := time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC)
	if !entries[0].at.Equal(want0) || entries[0].body != "Left voicemail, will try again tomorrow." {
		t.Errorf("entries[0] = %+v", entries[0])
	}
	want1 := time.Date(2026, 1, 14, 9, 15, 0, 0, time.UTC)
	if !entries[1].at.Equal(want1) || entries[1].body != "No answer." {
		t.Errorf("entries[1] = %+v", entries[1])
	}
}

func TestParseTimestampedLog_EmbeddedNewline(t *testing.T) {
	// A staff member's free-typed note can itself contain a newline --
	// the parser must not split on every \n, only at recognized
	// "<timestamp>: " anchors.
	raw := "2026-01-15 14:30:00: Line one\nLine two.\n2026-01-14 09:15:00: Single line.\n"
	entries, ok := parseTimestampedLog(raw)
	if !ok || len(entries) != 2 {
		t.Fatalf("ok=%v entries=%+v", ok, entries)
	}
	if entries[0].body != "Line one\nLine two." {
		t.Errorf("entries[0].body = %q, want %q", entries[0].body, "Line one\nLine two.")
	}
}

func TestParseTimestampedLog_Unrecognized(t *testing.T) {
	if _, ok := parseTimestampedLog("just some free text, no timestamps"); ok {
		t.Error("expected ok = false for text with no recognizable timestamp anchors")
	}
	if _, ok := parseTimestampedLog(""); ok {
		t.Error("expected ok = false for empty input")
	}
}

// TestParseTimestampedLog_AllEntriesFailToParse guards against a real
// regression: the anchor regex can match a syntactically-timestamp-
// shaped string whose date is nonsense (month 99), which time.Parse
// then rejects for every entry -- ok must come back false so the
// caller's raw-text fallback runs instead of silently importing
// nothing (see parseTimestampedLog's own comment on why).
func TestParseTimestampedLog_AllEntriesFailToParse(t *testing.T) {
	entries, ok := parseTimestampedLog("2026-99-99 14:30:00: important note.\n")
	if ok || len(entries) != 0 {
		t.Errorf("parseTimestampedLog(malformed timestamp) = %+v, %v; want nil, false", entries, ok)
	}
}
