-- name: CreateProtocol :one
insert into protocols (lab_id, name) values (sqlc.arg(lab_id), sqlc.arg(name))
returning *;

-- name: GetProtocolByID :one
select * from protocols where id = sqlc.arg(id);

-- name: ListProtocolsByLab :many
select * from protocols where lab_id = sqlc.arg(lab_id) order by name;

-- name: UpdateProtocol :one
update protocols set name = sqlc.arg(name) where id = sqlc.arg(id)
returning *;

-- name: DeactivateProtocol :exec
update protocols set deactivated_at = now() where id = sqlc.arg(id);
