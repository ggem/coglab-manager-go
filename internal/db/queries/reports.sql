-- name: NIHReportByCategory :many
-- Per-race_ethnicity-category enrollment counts, cross-tabbed by sex, over
-- 'arrived' appointments in a date range -- the current-shape NIH PHS
-- Inclusion Enrollment Report (built against the merged race_ethnicity[]
-- column, not legacy's old separate ethnicity/race/other_race split). A
-- child selecting more than one category is counted in each -- this is
-- per-category, not mutually exclusive, so rows don't sum to the total
-- (see NIHReportTotals for that).
select
    category,
    count(distinct children.id) filter (where children.sex = 'male') as male,
    count(distinct children.id) filter (where children.sex = 'female') as female,
    count(distinct children.id) filter (where children.sex = 'unknown') as unknown
from children
join appointments on appointments.child_id = children.id
join experiments on experiments.id = appointments.experiment_id
left join experiment_grants on experiment_grants.experiment_id = experiments.id
cross join lateral (select unnest(children.race_ethnicity)::text as category) as u
where experiments.lab_id = sqlc.arg(lab_id)
  and appointments.status = 'arrived'
  and appointments.schedule_date between sqlc.arg(start_date) and sqlc.arg(end_date)
  and (sqlc.narg(grant_id)::bigint is null or experiment_grants.grant_id = sqlc.narg(grant_id))
group by category
order by category;

-- name: NIHReportTotals :one
-- Distinct-child totals by sex, same filters as NIHReportByCategory --
-- a naive sum of the per-category rows would double-count a child who
-- selected more than one race_ethnicity category, so this is computed
-- separately rather than derived from the category rows.
select
    count(distinct children.id) filter (where children.sex = 'male') as male,
    count(distinct children.id) filter (where children.sex = 'female') as female,
    count(distinct children.id) filter (where children.sex = 'unknown') as unknown
from children
join appointments on appointments.child_id = children.id
join experiments on experiments.id = appointments.experiment_id
left join experiment_grants on experiment_grants.experiment_id = experiments.id
where experiments.lab_id = sqlc.arg(lab_id)
  and appointments.status = 'arrived'
  and appointments.schedule_date between sqlc.arg(start_date) and sqlc.arg(end_date)
  and (sqlc.narg(grant_id)::bigint is null or experiment_grants.grant_id = sqlc.narg(grant_id));

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
