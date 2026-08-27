-- name: CreateExperiment :one
insert into experiments (
    lab_id, name, description, sessions, age_range_min_months, age_range_max_months,
    start_date, end_date, status, duration_minutes, filter_premies,
    filter_min_languages, filter_languages
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
    sqlc.arg(filter_languages)
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
    filter_languages = sqlc.arg(filter_languages)
where id = sqlc.arg(id)
returning *;

-- name: DeactivateExperiment :exec
update experiments set deactivated_at = now() where id = sqlc.arg(id);

-- Join management: conditions, equipment, and training-role requirements
-- for an experiment. Each List query joins back to the lookup table so
-- callers get full rows (name, etc.) in one query instead of N+1 lookups.

-- name: AddExperimentCondition :exec
insert into experiment_conditions (experiment_id, condition_id)
values (sqlc.arg(experiment_id), sqlc.arg(condition_id));

-- name: RemoveExperimentCondition :exec
delete from experiment_conditions
where experiment_id = sqlc.arg(experiment_id) and condition_id = sqlc.arg(condition_id);

-- name: ListExperimentConditions :many
select conditions.* from conditions
join experiment_conditions on experiment_conditions.condition_id = conditions.id
where experiment_conditions.experiment_id = sqlc.arg(experiment_id)
order by conditions.id;

-- name: AddExperimentEquipment :exec
insert into experiment_equipment_requirements (experiment_id, equipment_id)
values (sqlc.arg(experiment_id), sqlc.arg(equipment_id));

-- name: RemoveExperimentEquipment :exec
delete from experiment_equipment_requirements
where experiment_id = sqlc.arg(experiment_id) and equipment_id = sqlc.arg(equipment_id);

-- name: ListExperimentEquipment :many
select equipment.* from equipment
join experiment_equipment_requirements on experiment_equipment_requirements.equipment_id = equipment.id
where experiment_equipment_requirements.experiment_id = sqlc.arg(experiment_id)
order by equipment.id;

-- name: AddExperimentTrainingRequirement :exec
insert into experiment_training_requirements (experiment_id, experiment_role_id)
values (sqlc.arg(experiment_id), sqlc.arg(experiment_role_id));

-- name: RemoveExperimentTrainingRequirement :exec
delete from experiment_training_requirements
where experiment_id = sqlc.arg(experiment_id) and experiment_role_id = sqlc.arg(experiment_role_id);

-- name: ListExperimentTrainingRequirements :many
select experiment_roles.* from experiment_roles
join experiment_training_requirements
    on experiment_training_requirements.experiment_role_id = experiment_roles.id
where experiment_training_requirements.experiment_id = sqlc.arg(experiment_id)
order by experiment_roles.id;
