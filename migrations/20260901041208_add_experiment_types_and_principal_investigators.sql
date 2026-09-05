-- +goose Up

-- A lab-configurable category shown on every experiment in legacy
-- (experiment_type_id); same shape as conditions/equipment/etc.
create table experiment_types (
    id             bigint generated always as identity primary key,
    lab_id         bigint not null references labs(id),
    name           text not null,
    deactivated_at timestamptz,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now()
);

create trigger experiment_types_set_updated_at
    before update on experiment_types
    for each row
    execute function set_updated_at();

alter table experiments add column experiment_type_id bigint references experiment_types(id);

-- Legacy's principal_investigators join table had no analog here --
-- same shape as experiment_grants.
create table experiment_principal_investigators (
    experiment_id bigint not null references experiments(id),
    user_id       bigint not null references users(id),
    primary key (experiment_id, user_id)
);

-- +goose Down
drop table experiment_principal_investigators;
alter table experiments drop column experiment_type_id;
drop trigger experiment_types_set_updated_at on experiment_types;
drop table experiment_types;
