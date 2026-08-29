-- name: CreateAppointment :one
insert into appointments (experiment_id, child_id, session, age_range_min_months, age_range_max_months, sibling_coming)
values (
    sqlc.arg(experiment_id), sqlc.arg(child_id), sqlc.arg(session),
    sqlc.narg(age_range_min_months), sqlc.narg(age_range_max_months), sqlc.arg(sibling_coming)
)
returning *;

-- name: GetAppointmentByID :one
select * from appointments where id = sqlc.arg(id);

-- name: ListAppointmentsByExperiment :many
select * from appointments
where experiment_id = sqlc.arg(experiment_id)
  and (sqlc.narg(status)::text is null or status = sqlc.narg(status))
order by created_at;

-- name: GetAppointmentLabID :one
-- For lab-membership authorization (mirrors GetConditionValueLabID):
-- appointments has no lab_id of its own, so resolve it via the experiment.
select experiments.lab_id from appointments
join experiments on experiments.id = appointments.experiment_id
where appointments.id = sqlc.arg(id);

-- name: ScheduleAppointment :one
-- Commits a chosen slot: the caller re-validates availability itself
-- immediately before calling this (defensive re-check against staleness,
-- since time passes between a search and a commit) -- this query just
-- writes the result.
update appointments
set schedule_date = sqlc.arg(schedule_date),
    schedule_time_start = sqlc.arg(schedule_time_start),
    schedule_time_end = sqlc.arg(schedule_time_end),
    status = 'pending'
where id = sqlc.arg(id)
returning *;

-- name: ReleaseAppointment :one
-- Deliberately allows releasing a 'pending' (already time-scheduled)
-- appointment, not just an unscheduled one -- matches legacy's per-child
-- release, which does the same. Frees the child up again: 'released'
-- falls outside appointments_one_active_hold_per_child's predicate.
update appointments
set status = 'released'
where id = sqlc.arg(id) and status in ('to_be_scheduled', 'pending')
returning *;

-- name: CreateAppointmentExperimenter :one
insert into appointment_experimenters (appointment_id, user_id, experiment_role_id, is_greeter)
values (sqlc.arg(appointment_id), sqlc.arg(user_id), sqlc.arg(experiment_role_id), sqlc.arg(is_greeter))
returning *;

-- name: ListAppointmentExperimenters :many
select * from appointment_experimenters where appointment_id = sqlc.arg(appointment_id);

-- name: ListBusyAppointmentExperimentersForDateRange :many
-- Members already committed to a Pending appointment within this date
-- range in this lab, with the date and time range they're busy -- one
-- query for a whole multi-day search rather than one call per candidate
-- day; the caller groups rows by date in Go.
select appointment_experimenters.user_id, appointments.schedule_date,
       appointments.schedule_time_start, appointments.schedule_time_end
from appointment_experimenters
join appointments on appointments.id = appointment_experimenters.appointment_id
join experiments on experiments.id = appointments.experiment_id
where experiments.lab_id = sqlc.arg(lab_id)
  and appointments.schedule_date between sqlc.arg(start_date) and sqlc.arg(end_date)
  and appointments.status = 'pending';

-- name: ListBusyEquipmentForDateRange :many
-- Equipment already required by a Pending appointment within this date
-- range in this lab, with the date and time range it's in use -- counted
-- against each equipment's quantity for that day's availability grid.
select experiment_equipment_requirements.equipment_id, appointments.schedule_date,
       appointments.schedule_time_start, appointments.schedule_time_end
from experiment_equipment_requirements
join appointments on appointments.experiment_id = experiment_equipment_requirements.experiment_id
join experiments on experiments.id = appointments.experiment_id
where experiments.lab_id = sqlc.arg(lab_id)
  and appointments.schedule_date between sqlc.arg(start_date) and sqlc.arg(end_date)
  and appointments.status = 'pending';
