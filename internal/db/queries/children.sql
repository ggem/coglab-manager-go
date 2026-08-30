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
    response = sqlc.arg(response),
    mcdi_percentile = sqlc.narg(mcdi_percentile),
    mcdi_date = sqlc.narg(mcdi_date)
where id = sqlc.arg(id)
returning *;

-- name: DeactivateChild :exec
update children set
    deactivated_at = now(),
    inactive_reason = sqlc.arg(inactive_reason)
where id = sqlc.arg(id);

-- name: SearchChildren :many
-- Every filter is optional (sqlc.narg(x) is null or ...); passing none
-- returns every child (subject to include_deactivated/limit_count).
--
-- Name matching uses pg_trgm's word_similarity rather than plain
-- similarity() or ILIKE: word_similarity scores how well the query matches
-- *part of* the field, so a short query like "jhon" can still match
-- "Johnathan" -- plain similarity() compares the whole strings and craters
-- when they're very different lengths, and ILIKE only catches exact
-- substrings, not typos. The 0.2 threshold is a literal in the query
-- rather than the pg_trgm.similarity_threshold GUC (the %/<% operators'
-- default is 0.3, too strict for short name fields, and overriding a GUC
-- per-query risks leaking to the next query on a pooled connection).
select *
from children
where
    (sqlc.narg(name_query)::text is null
        or word_similarity(sqlc.narg(name_query), first_name) > 0.2
        or word_similarity(sqlc.narg(name_query), last_name) > 0.2)
    and (sqlc.narg(min_birth_date)::date is null or birth_date >= sqlc.narg(min_birth_date))
    and (sqlc.narg(max_birth_date)::date is null or birth_date <= sqlc.narg(max_birth_date))
    and (sqlc.narg(sex)::text is null or sex = sqlc.narg(sex))
    and (sqlc.narg(twin)::boolean is null or twin = sqlc.narg(twin))
    and (sqlc.narg(premie)::boolean is null or premie = sqlc.narg(premie))
    and (sqlc.narg(language)::text is null or languages @> array[sqlc.narg(language)]::text[])
    and (sqlc.narg(family_id)::bigint is null or family_id = sqlc.narg(family_id))
    and (sqlc.arg(include_deactivated)::boolean or deactivated_at is null)
order by
    greatest(
        word_similarity(coalesce(sqlc.narg(name_query), ''), first_name),
        word_similarity(coalesce(sqlc.narg(name_query), ''), last_name)
    ) desc,
    last_name,
    first_name
limit sqlc.arg(limit_count);
