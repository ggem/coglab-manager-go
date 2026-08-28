-- name: CreateScheduleBlocking :one
insert into schedule_blockings (lab_id, date, start_time, end_time, reason)
values (sqlc.arg(lab_id), sqlc.arg(date), sqlc.arg(start_time), sqlc.arg(end_time), sqlc.arg(reason))
returning *;

-- name: GetScheduleBlockingByID :one
select * from schedule_blockings where id = sqlc.arg(id);

-- name: ListScheduleBlockingsByLab :many
select * from schedule_blockings
where lab_id = sqlc.arg(lab_id) and deactivated_at is null
order by date, start_time;

-- name: ListScheduleBlockingsForDateRange :many
-- One query for a whole multi-day search range, rather than one call per
-- candidate day -- the caller groups rows by date in Go.
select * from schedule_blockings
where lab_id = sqlc.arg(lab_id)
  and date between sqlc.arg(start_date) and sqlc.arg(end_date)
  and deactivated_at is null;

-- name: DeactivateScheduleBlocking :exec
update schedule_blockings set deactivated_at = now() where id = sqlc.arg(id);
