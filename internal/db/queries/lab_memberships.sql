-- name: GetLabMembership :one
select * from lab_memberships where user_id = sqlc.arg(user_id) and lab_id = sqlc.arg(lab_id);

-- name: ListLabsForUser :many
select labs.* from labs
join lab_memberships on lab_memberships.lab_id = labs.id
where lab_memberships.user_id = sqlc.arg(user_id)
order by labs.name;
