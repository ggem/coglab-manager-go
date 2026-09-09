-- name: GetLabMembership :one
select * from lab_memberships where user_id = sqlc.arg(user_id) and lab_id = sqlc.arg(lab_id);

-- name: ListLabsForUser :many
select labs.* from labs
join lab_memberships on lab_memberships.lab_id = labs.id
where lab_memberships.user_id = sqlc.arg(user_id)
order by labs.name;

-- name: ListLabMembers :many
-- The candidate pool a picker (e.g. principal investigators) draws
-- from -- full User rows, same "select users.*, filter deactivated"
-- shape as ListLabMemberTrainingsForRole.
select users.* from users
join lab_memberships on lab_memberships.user_id = users.id
where lab_memberships.lab_id = sqlc.arg(lab_id)
  and users.deactivated_at is null
order by users.last_name, users.first_name;

-- name: ListLabMembershipsForLab :many
-- The lab-members admin page's roster: membership details (permission
-- role, scheduling priority), not just the bare user rows ListLabMembers
-- (a different, existing query used by several AttachList pickers)
-- returns -- that one's response shape is relied on elsewhere and
-- shouldn't change.
select users.id as user_id, users.first_name, users.last_name, users.email,
       lab_memberships.role_id, roles.name as role_name, lab_memberships.priority
from lab_memberships
join users on users.id = lab_memberships.user_id
join roles on roles.id = lab_memberships.role_id
where lab_memberships.lab_id = sqlc.arg(lab_id)
  and users.deactivated_at is null
order by users.last_name, users.first_name;

-- name: CreateLabMembership :one
-- priority is deliberately omitted -- the column's own default
-- ('undergrad_no_project') applies, tuned afterward via
-- UpdateLabMembership. The table's unique(user_id, lab_id) constraint
-- rejects adding someone already a member (surfaces as a 409 via the
-- existing writeDBError conflict handling).
insert into lab_memberships (user_id, lab_id, role_id)
values (sqlc.arg(user_id), sqlc.arg(lab_id), sqlc.arg(role_id))
returning *;

-- name: UpdateLabMembership :one
update lab_memberships
set role_id = sqlc.arg(role_id), priority = sqlc.arg(priority)
where user_id = sqlc.arg(user_id) and lab_id = sqlc.arg(lab_id)
returning *;

-- name: RemoveLabMembership :one
-- A hard delete, not a deactivation -- lab_memberships has no
-- deactivated_at column. Returns the removed row (rather than :exec) so
-- the caller can 404 when nothing matched, and use the real
-- lab_memberships.id for its audit event -- consistent with
-- Create/UpdateLabMembership, which both use the actual row id, not the
-- user id.
delete from lab_memberships where user_id = sqlc.arg(user_id) and lab_id = sqlc.arg(lab_id)
returning *;

-- name: RemoveLabMemberTrainingsForUserInLab :exec
-- Run alongside RemoveLabMembership, in the same transaction: removing
-- someone from a lab should also retire them from that lab's studies,
-- not leave their trainings behind as a dangling, still-schedulable
-- candidate (see ListLabMemberTrainingsForRoleByPriority's LEFT JOIN,
-- which tolerates a trained user with no lab_memberships row for
-- legacy-import reasons -- that tolerance was never meant to cover a
-- live "remove this person" action producing the same shape on
-- purpose).
delete from lab_member_trainings
where user_id = sqlc.arg(user_id)
  and experiment_role_id in (select id from experiment_roles where lab_id = sqlc.arg(lab_id));

-- name: IsLabAdmin :one
-- Backs requireLabAdminFromURL: unlike plain lab membership (checked by
-- GetLabMembership), managing OTHER members -- creating, editing their
-- permission role and priority, or removing them -- requires the caller
-- to hold this lab's "admin" role, not just any membership.
select exists (
    select 1 from lab_memberships
    join roles on roles.id = lab_memberships.role_id
    where lab_memberships.user_id = sqlc.arg(user_id)
      and lab_memberships.lab_id = sqlc.arg(lab_id)
      and roles.name = 'admin'
);

-- name: SearchUsersNotInLab :many
-- Candidate pool for "add an existing person to this lab" -- same
-- word_similarity name-matching pattern as SearchChildren/
-- SearchFamilies (see children.sql), scoped to active users who
-- aren't already a member of this lab.
select users.*
from users
where
    (sqlc.narg(name_query)::text is null
        or word_similarity(sqlc.narg(name_query), first_name) > 0.2
        or word_similarity(sqlc.narg(name_query), last_name) > 0.2)
    and users.deactivated_at is null
    and not exists (
        select 1 from lab_memberships
        where lab_memberships.user_id = users.id and lab_memberships.lab_id = sqlc.arg(lab_id)
    )
order by users.last_name, users.first_name
limit 20;
