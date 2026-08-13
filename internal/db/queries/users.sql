-- name: CreateUser :one
insert into users (email, first_name, last_name, password_hash, is_platform_admin)
values (
    lower(sqlc.arg(email)),
    sqlc.arg(first_name),
    sqlc.arg(last_name),
    sqlc.narg(password_hash),
    sqlc.arg(is_platform_admin)
)
returning *;

-- name: GetUserByEmail :one
select * from users where lower(email) = lower(sqlc.arg(email));

-- name: GetUserByID :one
select * from users where id = sqlc.arg(id);
