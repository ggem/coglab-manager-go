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

-- name: SetExperimentRoleSitter :one
-- Dedicated action rather than part of UpdateExperimentRole: designating
-- the sitter role is a distinct decision from renaming a role. The
-- partial unique index (at most one sitter role per lab) rejects setting
-- a second role true while one's already set -- the caller must unset the
-- old one first, this doesn't swap automatically.
update experiment_roles set is_sitter_role = sqlc.arg(is_sitter_role)
where id = sqlc.arg(id)
returning *;

-- name: GetSitterRoleForLab :one
select * from experiment_roles where lab_id = sqlc.arg(lab_id) and is_sitter_role;
