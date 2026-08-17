-- name: CreateChild :one
insert into children (
    family_id, first_name, last_name, sex, birth_date, due_date,
    gestational_age_weeks, birth_weight, apgar_1, apgar_2, premie,
    birth_complications, twin, race_ethnicity, languages,
    recruitment_source_id, recruitment_source_other, response,
    created_by_user_id
) values (
    sqlc.arg(family_id),
    sqlc.arg(first_name),
    sqlc.arg(last_name),
    sqlc.arg(sex),
    sqlc.narg(birth_date),
    sqlc.narg(due_date),
    sqlc.narg(gestational_age_weeks),
    sqlc.narg(birth_weight),
    sqlc.narg(apgar_1),
    sqlc.narg(apgar_2),
    sqlc.narg(premie),
    sqlc.narg(birth_complications),
    sqlc.narg(twin),
    sqlc.arg(race_ethnicity),
    sqlc.arg(languages),
    sqlc.narg(recruitment_source_id),
    sqlc.arg(recruitment_source_other),
    sqlc.arg(response),
    sqlc.arg(created_by_user_id)
)
returning *;

-- name: GetChildByID :one
select * from children where id = sqlc.arg(id);

-- name: ListChildrenByFamily :many
select * from children where family_id = sqlc.arg(family_id) order by id;

-- name: UpdateChild :one
update children set
    first_name = sqlc.arg(first_name),
    last_name = sqlc.arg(last_name),
    sex = sqlc.arg(sex),
    birth_date = sqlc.narg(birth_date),
    due_date = sqlc.narg(due_date),
    gestational_age_weeks = sqlc.narg(gestational_age_weeks),
    birth_weight = sqlc.narg(birth_weight),
    apgar_1 = sqlc.narg(apgar_1),
    apgar_2 = sqlc.narg(apgar_2),
    premie = sqlc.narg(premie),
    birth_complications = sqlc.narg(birth_complications),
    twin = sqlc.narg(twin),
    race_ethnicity = sqlc.arg(race_ethnicity),
    languages = sqlc.arg(languages),
    recruitment_source_id = sqlc.narg(recruitment_source_id),
    recruitment_source_other = sqlc.arg(recruitment_source_other),
    response = sqlc.arg(response)
where id = sqlc.arg(id)
returning *;

-- name: DeactivateChild :exec
update children set
    deactivated_at = now(),
    inactive_reason = sqlc.arg(inactive_reason)
where id = sqlc.arg(id);
