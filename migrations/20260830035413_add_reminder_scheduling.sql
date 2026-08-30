-- +goose Up

-- A durable cursor per named scheduled job -- currently just the staff
-- schedule-change digest, which needs to know "since when" to look for
-- appointment-related audit_events. Durable rather than in-memory: a
-- server restart shouldn't re-notify already-digested changes or
-- silently skip a window.
create table scheduled_job_runs (
    job_name     text primary key,
    last_run_at  timestamptz not null
);

-- Per-appointment family-reminder de-dup flag: null until a reminder is
-- sent for that appointment, then stamped once. Unlike the digest above,
-- this needs no shared cursor -- each appointment's own flag is the
-- source of truth for "have I reminded this family yet."
alter table appointments add column reminder_sent_at timestamptz;

-- +goose Down
alter table appointments drop column reminder_sent_at;
drop table if exists scheduled_job_runs;
