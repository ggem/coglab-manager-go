-- +goose Up
-- +goose StatementBegin
create or replace function set_updated_at()
returns trigger as $$
begin
    new.updated_at = now();
    return new;
end;
$$ language plpgsql;
-- +goose StatementEnd

create table roles (
    id          bigint generated always as identity primary key,
    name        text not null unique,
    description text not null,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger roles_set_updated_at
    before update on roles
    for each row
    execute function set_updated_at();

create table labs (
    id          bigint generated always as identity primary key,
    name        text not null,
    short_name  text not null unique,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger labs_set_updated_at
    before update on labs
    for each row
    execute function set_updated_at();

create table users (
    id                 bigint generated always as identity primary key,
    email              text not null,
    first_name         text not null,
    last_name          text not null,
    password_hash      text,
    is_platform_admin  boolean not null default false,
    created_at         timestamptz not null default now(),
    updated_at         timestamptz not null default now(),
    deactivated_at     timestamptz
);

create unique index users_email_key on users (lower(email));

create trigger users_set_updated_at
    before update on users
    for each row
    execute function set_updated_at();

create table lab_memberships (
    id          bigint generated always as identity primary key,
    user_id     bigint not null references users(id),
    lab_id      bigint not null references labs(id),
    role_id     bigint not null references roles(id),
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    unique (user_id, lab_id)
);

create trigger lab_memberships_set_updated_at
    before update on lab_memberships
    for each row
    execute function set_updated_at();

create index lab_memberships_user_id_idx on lab_memberships (user_id);
create index lab_memberships_lab_id_idx on lab_memberships (lab_id);

create table sessions (
    id            bigint generated always as identity primary key,
    token_hash    bytea not null unique,
    user_id       bigint not null references users(id),
    created_at    timestamptz not null default now(),
    expires_at    timestamptz not null,
    last_seen_at  timestamptz not null default now(),
    ip_address    inet,
    user_agent    text,
    revoked_at    timestamptz
);

create index sessions_user_id_idx on sessions (user_id);
create index sessions_expires_at_idx on sessions (expires_at);

create table audit_events (
    id             bigint generated always as identity primary key,
    occurred_at    timestamptz not null default now(),
    actor_user_id  bigint references users(id),
    lab_id         bigint references labs(id),
    action         text not null,
    entity_type    text,
    entity_id      bigint,
    ip_address     inet,
    metadata       jsonb
);

create index audit_events_occurred_at_idx on audit_events (occurred_at);
create index audit_events_actor_user_id_idx on audit_events (actor_user_id);

insert into roles (name, description) values
    ('staff', 'Day-to-day scheduling and participant search'),
    ('coordinator', 'Release/hold overrides and reporting, per the documented Lab Coordinator responsibilities'),
    ('admin', 'Full lab configuration access');
-- +goose Down
drop table if exists audit_events;
drop table if exists sessions;
drop table if exists lab_memberships;
drop table if exists users;
drop table if exists labs;
drop table if exists roles;
drop function if exists set_updated_at();
