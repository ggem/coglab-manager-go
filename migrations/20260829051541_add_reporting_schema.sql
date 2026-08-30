-- +goose Up

-- 'arrived' is the one addition to the appointment lifecycle this
-- milestone needs: HRC/demographics reporting requires knowing a visit
-- actually happened, not just that it was scheduled. The rest of the
-- lifecycle (call logs, no-show, confirm, notes) stays deferred.
-- Moving to 'arrived' clears the hold for free: it falls outside
-- appointments_one_active_hold_per_child's ('to_be_scheduled', 'pending')
-- predicate, same as 'released' does.
alter table appointments drop constraint appointments_status_check;
alter table appointments add constraint appointments_status_check
    check (status in ('to_be_scheduled', 'pending', 'released', 'arrived'));

create table protocols (
    id          bigint generated always as identity primary key,
    lab_id      bigint not null references labs(id),
    name        text not null,
    deactivated_at timestamptz,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger protocols_set_updated_at
    before update on protocols
    for each row
    execute function set_updated_at();

create index protocols_lab_id_idx on protocols (lab_id);

-- One protocol per experiment (not many-to-many, unlike
-- conditions/equipment/grants below) -- matches how the legacy HRC
-- report attributes each experiment to a single IRB protocol.
alter table experiments add column protocol_id bigint references protocols(id);

create table grants (
    id          bigint generated always as identity primary key,
    lab_id      bigint not null references labs(id),
    name        text not null,
    deactivated_at timestamptz,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger grants_set_updated_at
    before update on grants
    for each row
    execute function set_updated_at();

create index grants_lab_id_idx on grants (lab_id);

create table experiment_grants (
    experiment_id  bigint not null references experiments(id),
    grant_id       bigint not null references grants(id),
    primary key (experiment_id, grant_id)
);

-- Lab-maintained recruiting-priority tier per zip code, for the zip
-- codes report. Not tied to appointments/experiments -- children remain
-- unscoped by lab (the established shared-participant-pool design), so
-- this table only supplies the priority annotation, not the underlying
-- child counts.
create table zipcodes (
    id          bigint generated always as identity primary key,
    lab_id      bigint not null references labs(id),
    zip_code    text not null,
    priority    text not null default '',
    deactivated_at timestamptz,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger zipcodes_set_updated_at
    before update on zipcodes
    for each row
    execute function set_updated_at();

create unique index zipcodes_one_active_per_lab_and_zip
    on zipcodes (lab_id, zip_code)
    where deactivated_at is null;

create table newsletters (
    id          bigint generated always as identity primary key,
    lab_id      bigint not null references labs(id),
    name        text not null,
    deactivated_at timestamptz,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create trigger newsletters_set_updated_at
    before update on newsletters
    for each row
    execute function set_updated_at();

create index newsletters_lab_id_idx on newsletters (lab_id);

-- Marks a guardian as having received a given newsletter -- the
-- newsletter export's "already sent" exclusion filter.
create table newsletters_parents (
    newsletter_id  bigint not null references newsletters(id),
    guardian_id    bigint not null references guardians(id),
    sent_at        timestamptz not null default now(),
    primary key (newsletter_id, guardian_id)
);

-- +goose Down
drop table if exists newsletters_parents;
drop table if exists newsletters;
drop table if exists zipcodes;
drop table if exists experiment_grants;
drop table if exists grants;
alter table experiments drop column protocol_id;
drop table if exists protocols;
alter table appointments drop constraint appointments_status_check;
alter table appointments add constraint appointments_status_check
    check (status in ('to_be_scheduled', 'pending', 'released'));
