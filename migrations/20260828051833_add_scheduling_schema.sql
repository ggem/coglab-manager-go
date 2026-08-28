-- +goose Up

-- One row per disjoint availability window, replacing the legacy schema's
-- fixed 3-range-per-row columns -- a person can declare as many windows as
-- they actually have, not just up to 3. Scoped per (user_id, lab_id)
-- rather than globally per user, since a person can belong to multiple
-- labs (lab_memberships) with different availability in each.
create table lab_availability_general (
    id          bigint generated always as identity primary key,
    user_id     bigint not null references users(id),
    lab_id      bigint not null references labs(id),
    weekday     smallint not null check (weekday between 0 and 6),
    start_time  time not null,
    end_time    time not null check (end_time > start_time),
    deactivated_at timestamptz,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger lab_availability_general_set_updated_at
    before update on lab_availability_general
    for each row
    execute function set_updated_at();

create index lab_availability_general_user_lab_idx
    on lab_availability_general (user_id, lab_id);

-- Overrides lab_availability_general for a specific date: if any rows
-- exist here for (user, lab, date), use only these for that date: don't
-- also merge in the general weekday schedule. Inherited from the legacy
-- app, this can't represent "generally available, but blocked this one
-- day" on its own -- schedule_blockings (lab-wide) is the tool for that;
-- there's no per-person one-off "unavailable today" without an explicit
-- window of some other kind. Same limitation the legacy app had.
create table lab_availability_specific (
    id          bigint generated always as identity primary key,
    user_id     bigint not null references users(id),
    lab_id      bigint not null references labs(id),
    date        date not null,
    start_time  time not null,
    end_time    time not null check (end_time > start_time),
    deactivated_at timestamptz,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger lab_availability_specific_set_updated_at
    before update on lab_availability_specific
    for each row
    execute function set_updated_at();

create index lab_availability_specific_user_lab_date_idx
    on lab_availability_specific (user_id, lab_id, date);

-- Lab-wide blackout windows (e.g. holidays) subtracted from every
-- member's availability on that date.
create table schedule_blockings (
    id          bigint generated always as identity primary key,
    lab_id      bigint not null references labs(id),
    date        date not null,
    start_time  time not null,
    end_time    time not null check (end_time > start_time),
    reason      text not null default '',
    deactivated_at timestamptz,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger schedule_blockings_set_updated_at
    before update on schedule_blockings
    for each row
    execute function set_updated_at();

create index schedule_blockings_lab_date_idx on schedule_blockings (lab_id, date);

-- One appointment = one child attending one session of one experiment.
-- Deliberately minimal: just enough for create -> search -> commit to a
-- pending slot. The rest of the legacy status enum (Arrived, No Show,
-- Canceled, Problem, Released), call logs, and notes are a separate,
-- deferred appointment-lifecycle milestone.
create table appointments (
    id                     bigint generated always as identity primary key,
    experiment_id          bigint not null references experiments(id),
    child_id               bigint not null references children(id),
    session                smallint not null default 1,
    -- Per-appointment override of the experiment's default eligibility
    -- window; null means "use the experiment's own age_range_min/max".
    age_range_min_months   numeric(6, 2),
    age_range_max_months   numeric(6, 2),
    -- Drives whether the scheduling search requires a sitter: hard
    -- requirement when 'coming', soft attempt when 'unknown' (not yet
    -- confirmed), skipped entirely when 'not_coming'.
    sibling_coming text not null default 'unknown'
        check (sibling_coming in ('unknown', 'coming', 'not_coming')),
    schedule_date          date,
    schedule_time_start    time,
    schedule_time_end      time,
    status text not null default 'to_be_scheduled'
        check (status in ('to_be_scheduled', 'pending')),
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger appointments_set_updated_at
    before update on appointments
    for each row
    execute function set_updated_at();

create index appointments_experiment_id_idx on appointments (experiment_id);
create index appointments_child_id_idx on appointments (child_id);
create index appointments_schedule_date_idx on appointments (schedule_date);

-- The staff assigned to a committed appointment, produced by the
-- scheduling search. is_greeter is a label, not a separate role search --
-- it marks whichever assigned member is designated to meet the family
-- (any of them can do it), set on at most one row per appointment.
create table appointment_experimenters (
    id                 bigint generated always as identity primary key,
    appointment_id     bigint not null references appointments(id),
    user_id            bigint not null references users(id),
    experiment_role_id bigint not null references experiment_roles(id),
    is_greeter         boolean not null default false,
    unique (appointment_id, user_id)
);

create unique index appointment_experimenters_one_greeter_per_appointment
    on appointment_experimenters (appointment_id)
    where is_greeter;

create index appointment_experimenters_appointment_id_idx
    on appointment_experimenters (appointment_id);

-- Which lab members are trained/qualified for which experiment_role --
-- this is the "trained-member pool" the scheduling search draws
-- candidates from per role. lab_id isn't duplicated here since it's
-- already implied by experiment_role_id -> experiment_roles.lab_id,
-- matching the plain-association-table pattern experiment_training_
-- requirements already uses.
create table lab_member_trainings (
    user_id            bigint not null references users(id),
    experiment_role_id bigint not null references experiment_roles(id),
    primary key (user_id, experiment_role_id)
);

-- +goose Down
drop table if exists lab_member_trainings;
drop table if exists appointment_experimenters;
drop table if exists appointments;
drop table if exists schedule_blockings;
drop table if exists lab_availability_specific;
drop table if exists lab_availability_general;
