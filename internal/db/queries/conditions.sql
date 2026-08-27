-- name: CreateCondition :one
insert into conditions (lab_id, name) values (sqlc.arg(lab_id), sqlc.arg(name))
returning *;

-- name: GetConditionByID :one
select * from conditions where id = sqlc.arg(id);

-- name: ListConditionsByLab :many
select * from conditions where lab_id = sqlc.arg(lab_id) order by name;

-- name: UpdateCondition :one
update conditions set name = sqlc.arg(name) where id = sqlc.arg(id)
returning *;

-- name: DeactivateCondition :exec
update conditions set deactivated_at = now() where id = sqlc.arg(id);

-- name: CreateConditionValue :one
insert into condition_values (condition_id, name) values (sqlc.arg(condition_id), sqlc.arg(name))
returning *;

-- name: ListConditionValuesByCondition :many
select * from condition_values where condition_id = sqlc.arg(condition_id) order by name;

-- name: UpdateConditionValue :one
update condition_values set name = sqlc.arg(name) where id = sqlc.arg(id)
returning *;

-- name: DeactivateConditionValue :exec
update condition_values set deactivated_at = now() where id = sqlc.arg(id);
