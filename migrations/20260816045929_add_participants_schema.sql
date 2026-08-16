-- +goose Up
create extension if not exists pg_trgm;

-- A family groups siblings and holds the household-level info the legacy
-- app stored once per family (address, preferred contact method), rather
-- than duplicating it across two hardcoded parent slots.
create table families (
    id                       bigint generated always as identity primary key,
    address                  text not null default '',
    city                     text not null default '',
    state                    text not null default '',
    zip                      text not null default '',
    preferred_contact_method text check (preferred_contact_method in (
        'home_phone', 'work_phone', 'mobile_phone', 'fax', 'email', 'snail_mail'
    )),
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger families_set_updated_at
    before update on families
    for each row
    execute function set_updated_at();

-- One row per guardian, so a family can have one, two, or more -- the
-- legacy schema hardcoded exactly two guardian slots per family.
create table guardians (
    id            bigint generated always as identity primary key,
    family_id     bigint not null references families(id),
    first_name    text not null default '',
    last_name     text not null default '',
    education     text not null default 'unknown' check (education in (
        'unknown', 'without_high_school_diploma', 'hs_grad_no_college',
        'hs_grad_some_college', 'degree_from_4yr_college_or_higher', 'left_blank'
    )),
    occupation    text not null default '',
    phone_number  text not null default '',
    phone_type    text check (phone_type in (
        'home', 'work', 'mobile', 'fax', 'pager', 'disconnected', 'other'
    )),
    email         text not null default '',
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);

create trigger guardians_set_updated_at
    before update on guardians
    for each row
    execute function set_updated_at();

create index guardians_family_id_idx on guardians (family_id);
create index guardians_first_name_trgm_idx on guardians using gin (first_name gin_trgm_ops);
create index guardians_last_name_trgm_idx on guardians using gin (last_name gin_trgm_ops);

-- Recruitment channels ("how did you hear about us") as data instead of a
-- hardcoded enum: the legacy list had grown to 26 install-specific values
-- over the years, each addition requiring a schema migration.
create table recruitment_sources (
    id      bigint generated always as identity primary key,
    name    text not null unique,
    active  boolean not null default true
);

create table children (
    id                       bigint generated always as identity primary key,
    family_id                bigint not null references families(id),
    first_name               text not null default '',
    last_name                text not null default '',
    sex                      text not null default 'unknown' check (sex in ('unknown', 'male', 'female')),
    birth_date               date,
    due_date                 date,
    gestational_age_weeks    numeric(5, 2),
    birth_weight             numeric(6, 2),
    apgar_1                  smallint,
    apgar_2                  smallint,
    -- Tri-state Unknown/Yes/No in the legacy schema becomes a plain
    -- nullable boolean: null already means "unknown" in SQL.
    premie                   boolean,
    birth_complications      boolean,
    twin                     boolean,
    -- NIH's 2024 revision merges race and ethnicity into a single
    -- "select all that apply" question, replacing the old separate
    -- Hispanic/not-Hispanic ethnicity field plus single-select race +
    -- other_race fallback. An empty array means not reported/unknown --
    -- same convention as the nullable boolean columns above.
    race_ethnicity text[] not null default '{}' check (race_ethnicity <@ array[
        'american_indian_or_alaska_native', 'asian', 'black_or_african_american',
        'hispanic_or_latino', 'middle_eastern_or_north_african',
        'native_hawaiian_or_pacific_islander', 'white'
    ]),
    languages               text[] not null default '{}',
    recruitment_source_id   bigint references recruitment_sources(id),
    recruitment_source_other text not null default '',
    response text not null default 'unknown' check (response in (
        'unknown', 'email', 'snail_mail', 'phone', 'web_page'
    )),
    created_by_user_id  bigint not null references users(id),
    deactivated_at      timestamptz,
    inactive_reason      text not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger children_set_updated_at
    before update on children
    for each row
    execute function set_updated_at();

create index children_family_id_idx on children (family_id);
create index children_birth_date_idx on children (birth_date);
create index children_first_name_trgm_idx on children using gin (first_name gin_trgm_ops);
create index children_last_name_trgm_idx on children using gin (last_name gin_trgm_ops);
create index children_languages_idx on children using gin (languages);

-- Generic, append-only notes -- one table instead of the legacy's five
-- near-identical notes_<entity> tables, each a single mutable text blob
-- with no author or timestamp per entry.
create table notes (
    id              bigint generated always as identity primary key,
    entity_type     text not null,
    entity_id       bigint not null,
    author_user_id  bigint not null references users(id),
    body            text not null,
    created_at      timestamptz not null default now()
);

create index notes_entity_idx on notes (entity_type, entity_id);

-- +goose Down
drop table if exists notes;
drop table if exists children;
drop table if exists recruitment_sources;
drop table if exists guardians;
drop table if exists families;
drop extension if exists pg_trgm;
