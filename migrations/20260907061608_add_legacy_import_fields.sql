-- +goose Up

-- Widens the appointment lifecycle with legacy's remaining outcome
-- statuses (No Show/Canceled/Problem) and adds the separate
-- data-quality field legacy tracks alongside scheduling status -- both
-- needed so the upcoming legacy data import doesn't lose or mis-fold
-- real appointment history. See M10 in the roadmap memory.
alter table appointments drop constraint appointments_status_check;
alter table appointments add constraint appointments_status_check
    check (status in ('to_be_scheduled', 'pending', 'released', 'arrived',
                       'no_show', 'canceled', 'problem'));

alter table appointments add column data_status text not null default 'no_data'
    check (data_status in ('ok', 'fuss', 'experimental_error',
                            'equipment_failure', 'other', 'extra', 'no_data'));

-- The Greeter uses this to identify a family they've never met (real
-- production usage: ~76% of appointments have it set).
alter table appointments add column type_of_car text not null default '';

-- How the PI keeps data anonymous while analyzing results -- not read
-- by this app, but must round-trip losslessly through import.
alter table appointments add column participant_number text not null default '';

-- Legacy's cross-experiment eligibility lists: a child in experiment X
-- can be barred from (or required to have been in) experiment Y.
create table experiment_exclusions (
    experiment_id           bigint not null references experiments(id),
    excluded_experiment_id  bigint not null references experiments(id),
    primary key (experiment_id, excluded_experiment_id)
);

create table experiment_inclusions (
    experiment_id           bigint not null references experiments(id),
    included_experiment_id  bigint not null references experiments(id),
    primary key (experiment_id, included_experiment_id)
);

-- End-of-appointment participant gift tracking. Staff currently check
-- a child's appointment history by hand to avoid giving a repeat
-- token -- this milestone just makes the data real and visible;
-- auto-suggesting an ungiven token is a deferred future enhancement.
create table tokens (
    id             bigint generated always as identity primary key,
    lab_id         bigint not null references labs(id),
    name           text not null,
    deactivated_at timestamptz,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now()
);

create trigger tokens_set_updated_at
    before update on tokens
    for each row
    execute function set_updated_at();

create index tokens_lab_id_idx on tokens (lab_id);

alter table appointments add column token_id bigint references tokens(id);

-- Legacy's ordered enum, labeled "Sib-sitting priority" in the legacy
-- UI but functionally a general scheduling preference: lower priority
-- is scheduled first (e.g. undergrads before grad students). Kept as
-- self-describing text, matching the education/response/phone_type
-- convention elsewhere, rather than a bare integer -- the scheduling
-- search maps this to a sort rank (see appointments_search.go).
alter table lab_memberships add column priority text not null
    default 'undergrad_no_project'
    check (priority in ('undergrad_no_project', 'undergrad_with_project',
                         'lab_coordinator', 'graduate_student', 'postdoc',
                         'lab_director'));

-- +goose Down
alter table lab_memberships drop column priority;
alter table appointments drop column token_id;
drop table if exists tokens;
drop table if exists experiment_inclusions;
drop table if exists experiment_exclusions;
alter table appointments drop column participant_number;
alter table appointments drop column type_of_car;
alter table appointments drop column data_status;
alter table appointments drop constraint appointments_status_check;
alter table appointments add constraint appointments_status_check
    check (status in ('to_be_scheduled', 'pending', 'released', 'arrived'));
