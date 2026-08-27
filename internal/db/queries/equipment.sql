-- name: CreateEquipment :one
insert into equipment (lab_id, name, quantity)
values (sqlc.arg(lab_id), sqlc.arg(name), sqlc.arg(quantity))
returning *;

-- name: GetEquipmentByID :one
select * from equipment where id = sqlc.arg(id);

-- name: ListEquipmentByLab :many
select * from equipment where lab_id = sqlc.arg(lab_id) order by name;

-- name: UpdateEquipment :one
update equipment set name = sqlc.arg(name), quantity = sqlc.arg(quantity)
where id = sqlc.arg(id)
returning *;

-- name: DeactivateEquipment :exec
update equipment set deactivated_at = now() where id = sqlc.arg(id);
