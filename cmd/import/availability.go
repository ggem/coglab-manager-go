package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// timeRangeCols is one of the up to 3 (start,end) column pairs legacy's
// fixed-3-ranges-per-row schedule tables carry -- see the scheduling
// migration's own comment on why the current schema is one row per
// disjoint range instead.
var timeRangeCols = [3][2]string{
	{"time_start_1", "time_end_1"},
	{"time_start_2", "time_end_2"},
	{"time_start_3", "time_end_3"},
}

// importLabAvailabilityGeneral, importLabAvailabilitySpecific, and
// importScheduleBlockings all assign a synthetic sequential id in read
// order rather than preserving a legacy id (none of these legacy tables
// has one of its own -- lab_member_schedule_general/specific have a
// composite (member_id, weekday|date) key covering up to 3 ranges each,
// and schedule_blockings has no key at all). A stable "order by" on the
// legacy SELECT keeps that assignment -- and therefore idempotency on a
// rerun -- deterministic across runs.
func importLabAvailabilityGeneral(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, labIDByMember map[int64]int64) error {
	rows, err := queryLegacy(ctx, legacyDB, "select * from lab_member_schedule_general order by member_id, weekday")
	if err != nil {
		return err
	}
	id := int64(0)
	for _, row := range rows {
		memberID := row.int64("member_id")
		// Legacy's schedule tables have no lab_id of their own -- a
		// member has exactly one lab in legacy, so their imported
		// lab_memberships row (built in importUsers) is the only
		// available source for the lab_id this app's availability
		// tables require.
		labID, ok := labIDByMember[memberID]
		if !ok {
			report.Error("lab_availability_general", memberID, fmt.Errorf("member %d has no imported lab_memberships row", memberID))
			continue
		}
		for _, cols := range timeRangeCols {
			start := row.nullTimeStr(cols[0])
			end := row.nullTimeStr(cols[1])
			if start == nil || end == nil {
				continue
			}
			id++
			fields := map[string]any{
				"id":         id,
				"user_id":    memberID,
				"lab_id":     labID,
				"weekday":    row.int64("weekday"),
				"start_time": *start,
				"end_time":   *end,
			}
			err := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
				return insertRow(ctx, sp, "lab_availability_general", fields)
			})
			if err != nil {
				report.Error("lab_availability_general", memberID, err)
				continue
			}
			report.Ok("lab_availability_general")
		}
	}
	return bumpSequence(ctx, tx, "lab_availability_general")
}

func importLabAvailabilitySpecific(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report, labIDByMember map[int64]int64) error {
	rows, err := queryLegacy(ctx, legacyDB, "select * from lab_member_schedule_specific order by member_id, date")
	if err != nil {
		return err
	}
	id := int64(0)
	for _, row := range rows {
		memberID := row.int64("member_id")
		labID, ok := labIDByMember[memberID]
		if !ok {
			report.Error("lab_availability_specific", memberID, fmt.Errorf("member %d has no imported lab_memberships row", memberID))
			continue
		}
		date := row.nullDate("date")
		if date == nil {
			continue
		}
		for _, cols := range timeRangeCols {
			start := row.nullTimeStr(cols[0])
			end := row.nullTimeStr(cols[1])
			if start == nil || end == nil {
				continue
			}
			id++
			fields := map[string]any{
				"id":         id,
				"user_id":    memberID,
				"lab_id":     labID,
				"date":       *date,
				"start_time": *start,
				"end_time":   *end,
			}
			err := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
				return insertRow(ctx, sp, "lab_availability_specific", fields)
			})
			if err != nil {
				report.Error("lab_availability_specific", memberID, err)
				continue
			}
			report.Ok("lab_availability_specific")
		}
	}
	return bumpSequence(ctx, tx, "lab_availability_specific")
}

func importScheduleBlockings(ctx context.Context, legacyDB *sql.DB, tx pgx.Tx, report *Report) error {
	rows, err := queryLegacy(ctx, legacyDB, "select * from schedule_blockings order by lab_id, date, time_start")
	if err != nil {
		return err
	}
	id := int64(0)
	for _, row := range rows {
		id++
		labID := row.int64("lab_id")
		fields := map[string]any{
			"id":         id,
			"lab_id":     labID,
			"date":       row.nullDate("date"),
			"start_time": row.nullTimeStr("time_start"),
			"end_time":   row.nullTimeStr("time_end"),
			"reason":     row.str("reason"),
		}
		err := withSavepoint(ctx, tx, func(sp pgx.Tx) error {
			return insertRow(ctx, sp, "schedule_blockings", fields)
		})
		if err != nil {
			report.Error("schedule_blockings", labID, err)
			continue
		}
		report.Ok("schedule_blockings")
	}
	return bumpSequence(ctx, tx, "schedule_blockings")
}
