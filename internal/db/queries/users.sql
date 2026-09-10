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

-- name: SetUserPassword :exec
update users set password_hash = sqlc.arg(password_hash) where id = sqlc.arg(id);

-- name: SetUserPlatformAdmin :one
-- :one (RETURNING), not :exec, so granting/revoking admin on a
-- nonexistent id 404s instead of silently reporting success.
update users set is_platform_admin = sqlc.arg(is_platform_admin) where id = sqlc.arg(id) returning *;

-- name: ListUsers :many
select * from users order by created_at;

-- name: DeactivateUser :one
-- :one (RETURNING), not :exec, so deactivating a nonexistent id 404s
-- instead of silently reporting success -- same reasoning as
-- RemoveLabMembership.
update users set deactivated_at = now() where id = sqlc.arg(id) returning *;
