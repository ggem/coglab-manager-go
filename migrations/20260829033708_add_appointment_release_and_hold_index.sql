-- +goose Up

-- Widen the status enum with 'released' -- the M6 hold/release workflow's
-- one addition to the appointment lifecycle. The rest of legacy's status
-- enum (Arrived, No Show, Canceled, Problem) stays deferred to the
-- full appointment-lifecycle milestone.
alter table appointments drop constraint appointments_status_check;
alter table appointments add constraint appointments_status_check
    check (status in ('to_be_scheduled', 'pending', 'released'));

-- A child is "held" iff they have a live (to_be_scheduled/pending)
-- appointment -- for *any* experiment, not just one -- so this single
-- partial unique index both defines what "held" means and enforces it:
-- at most one active appointment per child, matching legacy's
-- hold_date/hold_experiment_id semantics (a child can only be actively
-- pursued for one study at a time) but as a real constraint instead of
-- legacy's soft `where hold_date is null` update race. A concurrent hold
-- attempt on an already-held child now fails the INSERT with a unique
-- violation rather than silently double-booking.
create unique index appointments_one_active_hold_per_child
    on appointments (child_id)
    where status in ('to_be_scheduled', 'pending');

-- +goose Down
drop index appointments_one_active_hold_per_child;
alter table appointments drop constraint appointments_status_check;
alter table appointments add constraint appointments_status_check
    check (status in ('to_be_scheduled', 'pending'));
