-- name: CreateLabAvailabilityGeneral :one
insert into lab_availability_general (user_id, lab_id, weekday, start_time, end_time)
values (sqlc.arg(user_id), sqlc.arg(lab_id), sqlc.arg(weekday), sqlc.arg(start_time), sqlc.arg(end_time))
returning *;

-- name: GetLabAvailabilityGeneralByID :one
select * from lab_availability_general where id = sqlc.arg(id);

-- name: ListLabAvailabilityGeneralByUser :many
select * from lab_availability_general
where user_id = sqlc.arg(user_id) and lab_id = sqlc.arg(lab_id) and deactivated_at is null
order by weekday, start_time;

-- name: DeactivateLabAvailabilityGeneral :exec
update lab_availability_general set deactivated_at = now() where id = sqlc.arg(id);

-- name: ListLabAvailabilityGeneralByLab :many
-- Every active member's general availability windows in a lab, for every
-- weekday at once: general availability doesn't vary by date (it's a
-- weekly-recurring schedule, not tied to any particular range), so a
-- multi-day search needs this fetched only once, not once per candidate
-- day -- the caller groups by weekday and filters to its candidate
-- members in Go, and prefers a member's specific-date rows over these
-- when both exist for the date in question.
select * from lab_availability_general
where lab_id = sqlc.arg(lab_id) and deactivated_at is null;

-- name: CreateLabAvailabilitySpecific :one
insert into lab_availability_specific (user_id, lab_id, date, start_time, end_time)
values (sqlc.arg(user_id), sqlc.arg(lab_id), sqlc.arg(date), sqlc.arg(start_time), sqlc.arg(end_time))
returning *;

-- name: GetLabAvailabilitySpecificByID :one
select * from lab_availability_specific where id = sqlc.arg(id);

-- name: ListLabAvailabilitySpecificByUser :many
select * from lab_availability_specific
where user_id = sqlc.arg(user_id) and lab_id = sqlc.arg(lab_id) and deactivated_at is null
order by date, start_time;

-- name: DeactivateLabAvailabilitySpecific :exec
update lab_availability_specific set deactivated_at = now() where id = sqlc.arg(id);

-- name: ListLabAvailabilitySpecificForDateRange :many
-- One query for a whole multi-day search range, rather than one call per
-- candidate day -- the caller groups rows by date in Go.
select * from lab_availability_specific
where lab_id = sqlc.arg(lab_id)
  and date between sqlc.arg(start_date) and sqlc.arg(end_date)
  and deactivated_at is null;
