-- name: CreateGuardian :one
insert into guardians (family_id, first_name, last_name, education, occupation, phone_number, phone_type, email)
values (
    sqlc.arg(family_id),
    sqlc.arg(first_name),
    sqlc.arg(last_name),
    sqlc.arg(education),
    sqlc.arg(occupation),
    sqlc.arg(phone_number),
    sqlc.narg(phone_type),
    sqlc.arg(email)
)
returning *;

-- name: GetGuardianByID :one
select * from guardians where id = sqlc.arg(id);

-- name: ListGuardiansByFamily :many
select * from guardians where family_id = sqlc.arg(family_id) order by id;

-- name: UpdateGuardian :one
update guardians set
    first_name = sqlc.arg(first_name),
    last_name = sqlc.arg(last_name),
    education = sqlc.arg(education),
    occupation = sqlc.arg(occupation),
    phone_number = sqlc.arg(phone_number),
    phone_type = sqlc.narg(phone_type),
    email = sqlc.arg(email)
where id = sqlc.arg(id)
returning *;

-- name: DeactivateGuardian :exec
update guardians set deactivated_at = now() where id = sqlc.arg(id);
