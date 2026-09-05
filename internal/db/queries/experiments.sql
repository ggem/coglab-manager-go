-- name: CreateExperiment :one
insert into experiments (
    lab_id, name, description, sessions, age_range_min_months, age_range_max_months,
    start_date, end_date, status, duration_minutes, filter_premies,
    filter_min_languages, filter_languages, protocol_id, experiment_type_id
) values (
    sqlc.arg(lab_id),
    sqlc.arg(name),
    sqlc.arg(description),
    sqlc.arg(sessions),
    sqlc.arg(age_range_min_months),
    sqlc.arg(age_range_max_months),
    sqlc.narg(start_date),
    sqlc.narg(end_date),
    sqlc.arg(status),
    sqlc.arg(duration_minutes),
    sqlc.arg(filter_premies),
    sqlc.arg(filter_min_languages),
    sqlc.arg(filter_languages),
    sqlc.narg(protocol_id),
    sqlc.narg(experiment_type_id)
)
returning *;

-- name: GetExperimentByID :one
select * from experiments where id = sqlc.arg(id);

-- name: ListExperimentsByLab :many
select * from experiments where lab_id = sqlc.arg(lab_id) order by id;

-- name: UpdateExperiment :one
update experiments set
    name = sqlc.arg(name),
    description = sqlc.arg(description),
    sessions = sqlc.arg(sessions),
    age_range_min_months = sqlc.arg(age_range_min_months),
    age_range_max_months = sqlc.arg(age_range_max_months),
    start_date = sqlc.narg(start_date),
    end_date = sqlc.narg(end_date),
    status = sqlc.arg(status),
    duration_minutes = sqlc.arg(duration_minutes),
    filter_premies = sqlc.arg(filter_premies),
    filter_min_languages = sqlc.arg(filter_min_languages),
    filter_languages = sqlc.arg(filter_languages),
    protocol_id = sqlc.narg(protocol_id),
    experiment_type_id = sqlc.narg(experiment_type_id)
where id = sqlc.arg(id)
returning *;

-- name: DeactivateExperiment :exec
update experiments set deactivated_at = now() where id = sqlc.arg(id);

-- Join management: conditions, equipment, and training-role requirements
-- for an experiment. Each List query joins back to the lookup table so
-- callers get full rows (name, etc.) in one query instead of N+1 lookups.
--
-- Each Add* query below is an insert...select gated on an exists check
-- that the target row belongs to the same lab as the experiment --
-- request authorization (requireLabMemberForExperiment) only checks the
-- experiment's own lab, not whether the attached id belongs to it, so
-- without this a lab member could attach another lab's condition/
-- equipment/role/grant/user by guessing its id. :execrows lets the
-- handler tell "attached" (1 row) apart from "cross-lab or unknown id"
-- (0 rows, since the insert then has nothing to select) without a
-- separate existence query.

-- name: AddExperimentCondition :execrows
insert into experiment_conditions (experiment_id, condition_id)
select sqlc.arg(experiment_id), sqlc.arg(condition_id)
where exists (
    select 1 from experiments e
    join conditions c on c.lab_id = e.lab_id
    where e.id = sqlc.arg(experiment_id) and c.id = sqlc.arg(condition_id)
);

-- name: RemoveExperimentCondition :exec
delete from experiment_conditions
where experiment_id = sqlc.arg(experiment_id) and condition_id = sqlc.arg(condition_id);

-- name: ListExperimentConditions :many
select conditions.* from conditions
join experiment_conditions on experiment_conditions.condition_id = conditions.id
where experiment_conditions.experiment_id = sqlc.arg(experiment_id)
order by conditions.id;

-- name: AddExperimentEquipment :execrows
insert into experiment_equipment_requirements (experiment_id, equipment_id)
select sqlc.arg(experiment_id), sqlc.arg(equipment_id)
where exists (
    select 1 from experiments e
    join equipment eq on eq.lab_id = e.lab_id
    where e.id = sqlc.arg(experiment_id) and eq.id = sqlc.arg(equipment_id)
);

-- name: RemoveExperimentEquipment :exec
delete from experiment_equipment_requirements
where experiment_id = sqlc.arg(experiment_id) and equipment_id = sqlc.arg(equipment_id);

-- name: ListExperimentEquipment :many
select equipment.* from equipment
join experiment_equipment_requirements on experiment_equipment_requirements.equipment_id = equipment.id
where experiment_equipment_requirements.experiment_id = sqlc.arg(experiment_id)
order by equipment.id;

-- name: AddExperimentTrainingRequirement :execrows
insert into experiment_training_requirements (experiment_id, experiment_role_id)
select sqlc.arg(experiment_id), sqlc.arg(experiment_role_id)
where exists (
    select 1 from experiments e
    join experiment_roles er on er.lab_id = e.lab_id
    where e.id = sqlc.arg(experiment_id) and er.id = sqlc.arg(experiment_role_id)
);

-- name: RemoveExperimentTrainingRequirement :exec
delete from experiment_training_requirements
where experiment_id = sqlc.arg(experiment_id) and experiment_role_id = sqlc.arg(experiment_role_id);

-- name: ListExperimentTrainingRequirements :many
select experiment_roles.* from experiment_roles
join experiment_training_requirements
    on experiment_training_requirements.experiment_role_id = experiment_roles.id
where experiment_training_requirements.experiment_id = sqlc.arg(experiment_id)
order by experiment_roles.id;

-- name: AddExperimentGrant :execrows
insert into experiment_grants (experiment_id, grant_id)
select sqlc.arg(experiment_id), sqlc.arg(grant_id)
where exists (
    select 1 from experiments e
    join grants g on g.lab_id = e.lab_id
    where e.id = sqlc.arg(experiment_id) and g.id = sqlc.arg(grant_id)
);

-- name: RemoveExperimentGrant :exec
delete from experiment_grants
where experiment_id = sqlc.arg(experiment_id) and grant_id = sqlc.arg(grant_id);

-- name: ListExperimentGrants :many
select grants.* from grants
join experiment_grants on experiment_grants.grant_id = grants.id
where experiment_grants.experiment_id = sqlc.arg(experiment_id)
order by grants.id;

-- name: AddExperimentPrincipalInvestigator :execrows
insert into experiment_principal_investigators (experiment_id, user_id)
select sqlc.arg(experiment_id), sqlc.arg(user_id)
where exists (
    select 1 from experiments e
    join lab_memberships lm on lm.lab_id = e.lab_id
    where e.id = sqlc.arg(experiment_id) and lm.user_id = sqlc.arg(user_id)
);

-- name: RemoveExperimentPrincipalInvestigator :exec
delete from experiment_principal_investigators
where experiment_id = sqlc.arg(experiment_id) and user_id = sqlc.arg(user_id);

-- name: ListExperimentPrincipalInvestigators :many
select users.* from users
join experiment_principal_investigators
    on experiment_principal_investigators.user_id = users.id
where experiment_principal_investigators.experiment_id = sqlc.arg(experiment_id)
order by users.last_name, users.first_name;
