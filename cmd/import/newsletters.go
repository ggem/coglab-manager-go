package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// importNewsletters imports the newsletters lookup itself. Legacy's
// sent_date belongs to one specific mailing event, but this app's
// newsletters row represents a reusable named category (individual
// send events live in newsletters_parents.sent_at instead, per FM6's
// design) -- so sent_date has nowhere to go and is dropped; it isn't
// lost information about who received what, just about which single
// historical mailing a newsletter row was originally created for.
func importNewsletters(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, importedAt time.Time) (nameToID map[string]int64, err error) {
	rows, err := queryLegacy(ctx, legacyDB, "select * from newsletters")
	if err != nil {
		return nil, err
	}
	nameToID = make(map[string]int64, len(rows))
	for _, row := range rows {
		id := row.int64("newsletter_id")
		name := row.str("name")
		fields := map[string]any{
			"id":             id,
			"lab_id":         row.int64("lab_id"),
			"name":           name,
			"deactivated_at": deactivatedAt(row.str("record_status"), importedAt),
		}
		txErr := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			return insertRow(ctx, sp, "newsletters", fields)
		})
		if txErr != nil {
			report.Error("newsletters", id, txErr)
			continue
		}
		report.Ok("newsletters")
		nameToID[name] = id
	}
	if err := bumpSequence(ctx, tx, "newsletters"); err != nil {
		return nil, err
	}
	return nameToID, nil
}

// importNewsletterSent imports newsletters_parents. Despite its column
// name, legacy's newsletters_parents.newsletter_id is declared char(10)
// -- the same width as newsletters.name, not newsletters.newsletter_id
// -- so it holds the newsletter's *name*, not a real foreign key; this
// looks like a genuine legacy schema quirk (a denormalized name stored
// instead of the id), confirmed by the column's own declared type.
// Resolved via nameToID (from importNewsletters) rather than treated as
// a numeric id. parents_id maps to that family's first (lowest-slot)
// guardian, matching how the rest of this app already picks "the"
// guardian for a family-level legacy concept (see e.g. the demographics
// report's guardian_education, "order by guardians.id limit 1").
func importNewsletterSent(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, nameToID map[string]int64, firstGuardianByFamily map[int64]int64) error {
	rows, err := queryLegacy(ctx, legacyDB, "select * from newsletters_parents")
	if err != nil {
		return err
	}
	for _, row := range rows {
		parentsID := row.int64("parents_id")
		name := row.str("newsletter_id")

		newsletterID, ok := nameToID[name]
		if !ok {
			report.Error("newsletters_parents", parentsID, fmt.Errorf("unrecognized newsletter name %q", name))
			continue
		}
		guardianID, ok := firstGuardianByFamily[parentsID]
		if !ok {
			report.Error("newsletters_parents", parentsID, fmt.Errorf("family %d has no guardian to attribute this send to", parentsID))
			continue
		}

		fields := map[string]any{
			"newsletter_id": newsletterID,
			"guardian_id":   guardianID,
		}
		txErr := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			return insertAssoc(ctx, sp, "newsletters_parents", fields)
		})
		if txErr != nil {
			report.Error("newsletters_parents", parentsID, txErr)
			continue
		}
		report.Ok("newsletters_parents")
	}
	return nil
}
