-- name: CreateUser :one
insert into users (email, first_name, last_name, password_hash, is_platform_admin, sso_issuer, sso_subject)
values (
    lower(sqlc.arg(email)),
    sqlc.arg(first_name),
    sqlc.arg(last_name),
    sqlc.narg(password_hash),
    sqlc.arg(is_platform_admin),
    sqlc.narg(sso_issuer),
    sqlc.narg(sso_subject)
)
returning *;

-- name: GetUserByEmail :one
select * from users where lower(email) = lower(sqlc.arg(email));

-- name: GetUserByID :one
select * from users where id = sqlc.arg(id);

-- name: GetUserBySSOIdentity :one
select * from users where sso_issuer = sqlc.arg(sso_issuer) and sso_subject = sqlc.arg(sso_subject);

-- name: SetUserSSOIdentity :exec
-- Backfills the link the first time an existing local-password account
-- signs in via SSO -- matched by email at that point, not (issuer, sub)
-- (which didn't exist on the row yet).
update users set sso_issuer = sqlc.arg(sso_issuer), sso_subject = sqlc.arg(sso_subject) where id = sqlc.arg(id);
