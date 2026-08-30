// Package reminders holds the two scheduled email jobs that replace
// legacy's cron+curl mechanism: RunStaffDigest (a staff member's daily
// "your schedule changed" summary, ported from a real legacy feature) and
// RunFamilyReminders (a new feature with no legacy precedent -- families
// get an informational reminder before their scheduled appointment).
//
// Both are plain functions taking an explicit now time.Time rather than
// calling time.Now() internally, so tests control time without sleeping.
// Scheduler (scheduler.go) is the thin goroutine/ticker wrapper that
// calls them on a real clock; it has no business logic of its own to
// keep the goroutine/timer machinery separate from what's actually being
// tested.
package reminders
