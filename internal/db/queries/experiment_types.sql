-- name: CreateExperimentType :one
insert into experiment_types (lab_id, name) values (sqlc.arg(lab_id), sqlc.arg(name))
returning *;

-- name: GetExperimentTypeByID :one
select * from experiment_types where id = sqlc.arg(id);

-- name: ListExperimentTypesByLab :many
select * from experiment_types where lab_id = sqlc.arg(lab_id) order by name;

-- name: UpdateExperimentType :one
update experiment_types set name = sqlc.arg(name) where id = sqlc.arg(id)
returning *;

-- name: DeactivateExperimentType :exec
update experiment_types set deactivated_at = now() where id = sqlc.arg(id);
