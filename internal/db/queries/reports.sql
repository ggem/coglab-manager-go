-- name: NIHParticipantReport :many
-- One row per distinct child with an 'arrived' appointment in range --
-- the current-shape NIH participant-level data template, which must be
-- submitted as a flat CSV (one row per participant) rather than the
-- old aggregate category/sex crosstab. Race/ethnicity/sex label
-- mapping and age-unit computation happen in Go (see nih_report.go),
-- not here, so a null birth_date can be handled explicitly rather than
-- relying on sqlc's nullability inference over a computed expression.
-- When a child has more than one qualifying appointment in the window,
-- the earliest one is used for age-at-visit (distinct on + order by
-- schedule_date), matching the distinct-child counting the old
-- aggregate queries already used.
select distinct on (children.id)
    children.id as child_id,
    children.sex,
    children.race_ethnicity,
    children.birth_date,
    appointments.schedule_date
from children
join appointments on appointments.child_id = children.id
join experiments on experiments.id = appointments.experiment_id
left join experiment_grants on experiment_grants.experiment_id = experiments.id
where experiments.lab_id = sqlc.arg(lab_id)
  and appointments.status = 'arrived'
  and appointments.schedule_date between sqlc.arg(start_date) and sqlc.arg(end_date)
  and (sqlc.narg(grant_id)::bigint is null or experiment_grants.grant_id = sqlc.narg(grant_id))
order by children.id, appointments.schedule_date;

-- name: HRCReportByProtocol :many
-- Distinct-child 'arrived' counts per protocol in a date range, for the
-- lab's Human Research Committee (IRB) reporting.
select
    protocols.id as protocol_id,
    protocols.name as protocol_name,
    count(distinct appointments.child_id) as child_count
from protocols
left join experiments on experiments.protocol_id = protocols.id
left join appointments on appointments.experiment_id = experiments.id
    and appointments.status = 'arrived'
    and appointments.schedule_date between sqlc.arg(start_date) and sqlc.arg(end_date)
where protocols.lab_id = sqlc.arg(lab_id)
group by protocols.id, protocols.name
order by protocols.name;

-- name: HRCReportTotal :one
-- All-protocols total for the same window, lab-wide (including
-- experiments with no protocol assigned).
select count(distinct appointments.child_id) as child_count
from appointments
join experiments on experiments.id = appointments.experiment_id
where experiments.lab_id = sqlc.arg(lab_id)
  and appointments.status = 'arrived'
  and appointments.schedule_date between sqlc.arg(start_date) and sqlc.arg(end_date);

-- name: DemographicsReport :many
-- Per-child listing of 'arrived' appointments for one experiment in a
-- date range, with age at the appointment (in months) and the family's
-- first (lowest-id) guardian's education level. Handler computes summary
-- counts over these rows in Go rather than a second query.
select
    children.id as child_id,
    children.first_name,
    children.last_name,
    children.sex,
    children.race_ethnicity,
    appointments.schedule_date,
    (extract(year from age(appointments.schedule_date, children.birth_date)) * 12
        + extract(month from age(appointments.schedule_date, children.birth_date)))::float8 as age_months,
    coalesce(
        (select guardians.education from guardians
         where guardians.family_id = children.family_id
         order by guardians.id
         limit 1),
        'unknown'
    )::text as guardian_education
from children
join appointments on appointments.child_id = children.id
where appointments.experiment_id = sqlc.arg(experiment_id)
  and appointments.status = 'arrived'
  and appointments.schedule_date between sqlc.arg(start_date) and sqlc.arg(end_date)
order by appointments.schedule_date;

-- name: ZipCodesReport :many
-- Child counts by mailing zip code, optionally filtered by recruitment
-- source, annotated with the lab's recruiting-priority tier for that zip
-- when one is configured. Child counts are computed globally (children
-- aren't lab-scoped -- the established shared-participant-pool design);
-- only the priority annotation comes from this lab's zipcodes lookup.
select
    families.zip,
    zipcodes.priority,
    count(*) as child_count
from children
join families on families.id = children.family_id
left join zipcodes on zipcodes.zip_code = families.zip
    and zipcodes.lab_id = sqlc.arg(lab_id)
    and zipcodes.deactivated_at is null
where children.deactivated_at is null
  and (sqlc.narg(recruitment_source_id)::bigint is null or children.recruitment_source_id = sqlc.narg(recruitment_source_id))
group by families.zip, zipcodes.priority
order by families.zip;
