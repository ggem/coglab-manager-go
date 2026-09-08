-- +goose Up
alter table experiment_roles add column is_greeter_role boolean not null default false;

-- Mirrors experiment_roles_one_sitter_per_lab: at most one role per lab
-- can be designated the dedicated-greeter role -- an explicit,
-- renameable per-lab designation, not a hardcoded ID or reserved name.
-- Used by the scheduling engine when an appointment requests a
-- dedicated greeter (a real, intentional lab practice for appointments
-- where the study experimenter can't leave stimuli unattended to answer
-- the door themselves).
create unique index experiment_roles_one_greeter_per_lab
    on experiment_roles (lab_id)
    where is_greeter_role;

-- Persists like sibling_coming: set once by staff, stays in effect
-- across re-searches/reschedules rather than being a per-search-only
-- checkbox.
alter table appointments add column wants_dedicated_greeter boolean not null default false;

-- +goose Down
alter table appointments drop column wants_dedicated_greeter;
drop index experiment_roles_one_greeter_per_lab;
alter table experiment_roles drop column is_greeter_role;
