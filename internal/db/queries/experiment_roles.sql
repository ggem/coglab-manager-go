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
-- Also clears is_sitter_role/is_greeter_role: a deactivated role retiring
-- from the lab's normal training-requirement machinery should retire
-- from these designations too, rather than leaving a dangling "this
-- retired role is still the lab's sitter/greeter" state that only a raw
-- DB query would reveal (GetSitterRoleForLab/GetGreeterRoleForLab below
-- also filter deactivated_at as defense in depth, but a lab should never
-- end up needing that filter to matter).
update experiment_roles set deactivated_at = now(), is_sitter_role = false, is_greeter_role = false
where id = sqlc.arg(id);

-- name: SetExperimentRoleSitter :one
-- Dedicated action rather than part of UpdateExperimentRole: designating
-- the sitter role is a distinct decision from renaming a role. The
-- partial unique index (at most one sitter role per lab) rejects setting
-- a second role true while one's already set -- the caller must unset the
-- old one first, this doesn't swap automatically. Setting true is
-- rejected (zero rows) for an already-deactivated role -- the caller
-- (handleSetExperimentRoleSitter) checks this first for a clean 400
-- rather than a confusing 404; unsetting (false) is always allowed
-- regardless of deactivated status, to clean up any stale flag.
update experiment_roles set is_sitter_role = sqlc.arg(is_sitter_role)
where id = sqlc.arg(id)
  and (sqlc.arg(is_sitter_role) = false or deactivated_at is null)
returning *;

-- name: GetSitterRoleForLab :one
select * from experiment_roles where lab_id = sqlc.arg(lab_id) and is_sitter_role and deactivated_at is null;

-- name: SetExperimentRoleGreeter :one
-- Mirrors SetExperimentRoleSitter: a dedicated action, not part of
-- UpdateExperimentRole, with its own constraint (at most one greeter
-- role per lab, enforced by a partial unique index), the same
-- deactivated-role guard on setting true, and the same
-- always-allow-unset behavior.
update experiment_roles set is_greeter_role = sqlc.arg(is_greeter_role)
where id = sqlc.arg(id)
  and (sqlc.arg(is_greeter_role) = false or deactivated_at is null)
returning *;

-- name: GetGreeterRoleForLab :one
select * from experiment_roles where lab_id = sqlc.arg(lab_id) and is_greeter_role and deactivated_at is null;
