-- name: CreateNewsletter :one
insert into newsletters (lab_id, name) values (sqlc.arg(lab_id), sqlc.arg(name))
returning *;

-- name: GetNewsletterByID :one
select * from newsletters where id = sqlc.arg(id);

-- name: ListNewslettersByLab :many
select * from newsletters where lab_id = sqlc.arg(lab_id) order by name;

-- name: DeactivateNewsletter :exec
update newsletters set deactivated_at = now() where id = sqlc.arg(id);

-- name: ListEligibleFamiliesForNewsletter :many
-- One row per family with an 'arrived' appointment for this lab in the
-- window, represented by its lowest-id guardian (matching legacy's
-- single-guardian-per-row mail-merge shape). When newsletter_id is
-- given, excludes a (family, guardian) pairing already marked sent for
-- that newsletter -- note this can fall through to a different
-- (higher-id) guardian in the same family if the lowest-id one already
-- received it, rather than dropping the family outright.
select distinct on (families.id)
    families.id as family_id,
    guardians.id as guardian_id,
    guardians.first_name,
    guardians.last_name,
    families.address,
    families.city,
    families.state,
    families.zip
from families
join children on children.family_id = families.id
join appointments on appointments.child_id = children.id
join experiments on experiments.id = appointments.experiment_id
join guardians on guardians.family_id = families.id
where experiments.lab_id = sqlc.arg(lab_id)
  and appointments.status = 'arrived'
  and appointments.schedule_date between sqlc.arg(start_date) and sqlc.arg(end_date)
  and (sqlc.narg(newsletter_id)::bigint is null or not exists (
      select 1 from newsletters_parents
      where newsletters_parents.newsletter_id = sqlc.narg(newsletter_id)
        and newsletters_parents.guardian_id = guardians.id
  ))
order by families.id, guardians.id;

-- name: MarkNewsletterSent :exec
insert into newsletters_parents (newsletter_id, guardian_id)
values (sqlc.arg(newsletter_id), sqlc.arg(guardian_id))
on conflict do nothing;
