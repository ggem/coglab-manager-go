-- +goose Up
-- ListAppointmentsDueForReminder (internal/db/queries/reminders.sql)
-- runs every 30 minutes for the life of the deployment, filtering the
-- whole appointments table down to a handful of rows every time --
-- exactly the shape a partial expression index is for. The predicate
-- and expression here match that query's WHERE clause exactly, so the
-- planner can satisfy it with an index range scan instead of a full
-- table scan as the table keeps growing (already ~7,700 rows after the
-- M10 legacy import alone).
create index appointments_due_for_reminder
    on appointments (((schedule_date + schedule_time_start)::timestamp))
    where status = 'pending' and reminder_sent_at is null;

-- +goose Down
drop index appointments_due_for_reminder;
