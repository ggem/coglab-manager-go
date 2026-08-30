-- name: CreateGrant :one
insert into grants (lab_id, name) values (sqlc.arg(lab_id), sqlc.arg(name))
returning *;

-- name: GetGrantByID :one
select * from grants where id = sqlc.arg(id);

-- name: ListGrantsByLab :many
select * from grants where lab_id = sqlc.arg(lab_id) order by name;

-- name: UpdateGrant :one
update grants set name = sqlc.arg(name) where id = sqlc.arg(id)
returning *;

-- name: DeactivateGrant :exec
update grants set deactivated_at = now() where id = sqlc.arg(id);
