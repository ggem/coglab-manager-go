-- name: GetJobLastRun :one
select last_run_at from scheduled_job_runs where job_name = sqlc.arg(job_name);

-- name: UpsertJobLastRun :exec
-- last_run_at is always Postgres's own now(), not a caller-supplied
-- timestamp: it gets compared against audit_events.occurred_at (also
-- stamped by Postgres's now()) in ListChangedAppointmentIDsSince, and
-- comparing a value from the calling process's clock against one from
-- Postgres's clock is vulnerable to real (if small) clock skew between
-- the two -- caught by TestStaffDigestFlow_Integration flaking on
-- exactly this before this query was written this way.
insert into scheduled_job_runs (job_name, last_run_at)
values (sqlc.arg(job_name), now())
on conflict (job_name) do update set last_run_at = now();

-- name: ListChangedAppointmentIDsSince :many
-- Distinct appointment IDs with a relevant audit event since the digest's
-- last run -- replaces legacy's change_status dirty-flag column with a
-- query over the audit log this project already maintains as a hard
-- requirement. entity_id is forced non-null via coalesce (safe: the
-- where clause already excludes null entity_id) so the Go side gets a
-- plain int64, not a pointer.
select coalesce(entity_id, 0)::bigint as appointment_id
from audit_events
where entity_type = 'appointment'
  and action = any(sqlc.arg(actions)::text[])
  and occurred_at > sqlc.arg(since)
  and entity_id is not null
group by entity_id;

-- name: ListRecipientsForAppointments :many
-- Distinct (staff member, lab) pairs assigned to any of the given
-- appointments -- the digest's recipients. Recipients come from
-- appointment_experimenters (who's actually assigned), not from the
-- audit event's actor (who might be an admin scheduling on someone
-- else's behalf).
select distinct
    users.id as user_id,
    users.email,
    users.first_name,
    users.last_name,
    experiments.lab_id,
    labs.short_name as lab_short_name
from appointment_experimenters
join appointments on appointments.id = appointment_experimenters.appointment_id
join experiments on experiments.id = appointments.experiment_id
join labs on labs.id = experiments.lab_id
join users on users.id = appointment_experimenters.user_id
where appointment_experimenters.appointment_id = any(sqlc.arg(appointment_ids)::bigint[]);

-- name: ListPendingAppointmentsForUserInLab :many
-- One recipient's current upcoming Pending schedule in one lab -- the
-- digest email body. Filters to appointments the given user is assigned
-- to (via exists), but aggregates role names across every experimenter
-- on the appointment, not just the recipient's own role, matching
-- legacy's "(roles)" showing the whole assigned team for context.
select
    appointments.id as appointment_id,
    appointments.schedule_date,
    appointments.schedule_time_start,
    experiments.name as experiment_name,
    children.first_name as child_first_name,
    children.last_name as child_last_name,
    string_agg(distinct experiment_roles.name, ', ')::text as role_names
from appointments
join experiments on experiments.id = appointments.experiment_id
join children on children.id = appointments.child_id
join appointment_experimenters on appointment_experimenters.appointment_id = appointments.id
join experiment_roles on experiment_roles.id = appointment_experimenters.experiment_role_id
where experiments.lab_id = sqlc.arg(lab_id)
  and appointments.status = 'pending'
  and appointments.schedule_date >= sqlc.arg(today)
  and exists (
      select 1 from appointment_experimenters assigned
      where assigned.appointment_id = appointments.id and assigned.user_id = sqlc.arg(user_id)
  )
group by appointments.id, appointments.schedule_date, appointments.schedule_time_start,
         experiments.name, children.first_name, children.last_name
order by appointments.schedule_date, appointments.schedule_time_start;

-- name: ListAppointmentsDueForReminder :many
-- Pending, not-yet-reminded appointments starting at or before the given
-- cutoff (now + lead time), with the family's representative (lowest-id)
-- guardian -- same idiom ListEligibleFamiliesForNewsletter uses. A
-- family with no guardians at all (guardian_email null) is left for the
-- caller to skip and log, not silently dropped here.
select
    appointments.id as appointment_id,
    appointments.schedule_date,
    appointments.schedule_time_start,
    experiments.name as experiment_name,
    children.first_name as child_first_name,
    children.last_name as child_last_name,
    -- coalesced to '' rather than left nullable: sqlc's static analysis
    -- doesn't propagate the nullability a LEFT JOIN LATERAL introduces
    -- when a family has zero guardians, so an uncoalesced column here
    -- would generate a plain (non-pointer) Go string that panics on scan
    -- when the row is genuinely NULL. '' also matches this codebase's
    -- existing "empty string means absent" convention for guardian
    -- fields (see guardians.email's own not-null-default-'' column).
    coalesce(guardian.email, '')::text as guardian_email,
    coalesce(guardian.first_name, '')::text as guardian_first_name,
    coalesce(guardian.last_name, '')::text as guardian_last_name
from appointments
join experiments on experiments.id = appointments.experiment_id
join children on children.id = appointments.child_id
join families on families.id = children.family_id
left join lateral (
    select g.email, g.first_name, g.last_name
    from guardians g
    where g.family_id = families.id
    order by g.id
    limit 1
) as guardian on true
where appointments.status = 'pending'
  and appointments.reminder_sent_at is null
  and (appointments.schedule_date + appointments.schedule_time_start)::timestamp <= sqlc.arg(due_before)::timestamp
order by appointments.schedule_date, appointments.schedule_time_start;

-- name: MarkAppointmentReminderSent :exec
update appointments set reminder_sent_at = now() where id = sqlc.arg(id);
