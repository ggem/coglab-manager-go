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
