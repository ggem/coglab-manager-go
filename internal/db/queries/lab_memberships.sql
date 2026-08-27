-- name: GetLabMembership :one
select * from lab_memberships where user_id = sqlc.arg(user_id) and lab_id = sqlc.arg(lab_id);
