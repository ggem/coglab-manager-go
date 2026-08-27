-- +goose Up

-- Unlike families/children, experiments are lab-scoped: each lab runs its
-- own experiments, and (per M3's scope) so are the lookup tables below.
create table experiments (
    id                     bigint generated always as identity primary key,
    lab_id                 bigint not null references labs(id),
    name                   text not null default '',
    description            text not null default '',
    sessions               smallint not null default 1,
    age_range_min_months   numeric(6, 2) not null,
    age_range_max_months   numeric(6, 2) not null,
    start_date             date,
    end_date               date,
    status text not null default 'not_run' check (status in ('not_run', 'pilot', 'run')),
    duration_minutes       smallint not null default 60,
    filter_premies         boolean not null default true,
    filter_min_languages   smallint not null default 0,
    filter_languages       text[] not null default '{}',
    deactivated_at         timestamptz,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger experiments_set_updated_at
    before update on experiments
    for each row
    execute function set_updated_at();

create index experiments_lab_id_idx on experiments (lab_id);

create table conditions (
    id          bigint generated always as identity primary key,
    lab_id      bigint not null references labs(id),
    name        text not null,
    deactivated_at timestamptz,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger conditions_set_updated_at
    before update on conditions
    for each row
    execute function set_updated_at();

create index conditions_lab_id_idx on conditions (lab_id);

create table condition_values (
    id            bigint generated always as identity primary key,
    condition_id  bigint not null references conditions(id),
    name          text not null,
    deactivated_at timestamptz,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);

create trigger condition_values_set_updated_at
    before update on condition_values
    for each row
    execute function set_updated_at();

create index condition_values_condition_id_idx on condition_values (condition_id);

create table equipment (
    id          bigint generated always as identity primary key,
    lab_id      bigint not null references labs(id),
    name        text not null,
    quantity    smallint not null default 1,
    deactivated_at timestamptz,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger equipment_set_updated_at
    before update on equipment
    for each row
    execute function set_updated_at();

create index equipment_lab_id_idx on equipment (lab_id);

-- Training/qualification a lab member needs to run an experiment (e.g.
-- "Experimenter", "Coder") -- named experiment_roles, not roles, since
-- that name is already taken by the unrelated lab-membership permission
-- levels in the core schema migration.
create table experiment_roles (
    id          bigint generated always as identity primary key,
    lab_id      bigint not null references labs(id),
    name        text not null,
    deactivated_at timestamptz,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger experiment_roles_set_updated_at
    before update on experiment_roles
    for each row
    execute function set_updated_at();

create index experiment_roles_lab_id_idx on experiment_roles (lab_id);

-- Plain association tables, matching the shape of the legacy join tables
-- they replace: no soft delete of their own (unlinking is just removing
-- the row), no updated_at (nothing on the row can change but its
-- existence).
create table experiment_conditions (
    experiment_id  bigint not null references experiments(id),
    condition_id   bigint not null references conditions(id),
    primary key (experiment_id, condition_id)
);

create table experiment_equipment_requirements (
    experiment_id  bigint not null references experiments(id),
    equipment_id   bigint not null references equipment(id),
    primary key (experiment_id, equipment_id)
);

create table experiment_training_requirements (
    experiment_id      bigint not null references experiments(id),
    experiment_role_id bigint not null references experiment_roles(id),
    primary key (experiment_id, experiment_role_id)
);

-- +goose Down
drop table if exists experiment_training_requirements;
drop table if exists experiment_equipment_requirements;
drop table if exists experiment_conditions;
drop table if exists experiment_roles;
drop table if exists equipment;
drop table if exists condition_values;
drop table if exists conditions;
drop table if exists experiments;
