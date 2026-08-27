-- name: CreateExperimentRole :one
insert into experiment_roles (lab_id, name) values (sqlc.arg(lab_id), sqlc.arg(name))
returning *;

-- name: GetExperimentRoleByID :one
select * from experiment_roles where id = sqlc.arg(id);

-- name: ListExperimentRolesByLab :many
select * from experiment_roles where lab_id = sqlc.arg(lab_id) order by name;

-- name: UpdateExperimentRole :one
update experiment_roles set name = sqlc.arg(name) where id = sqlc.arg(id)
returning *;

-- name: DeactivateExperimentRole :exec
update experiment_roles set deactivated_at = now() where id = sqlc.arg(id);
