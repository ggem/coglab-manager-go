-- name: CreateSession :one
insert into sessions (token_hash, user_id, expires_at, ip_address, user_agent)
values (
    sqlc.arg(token_hash),
    sqlc.arg(user_id),
    sqlc.arg(expires_at),
    sqlc.narg(ip_address),
    sqlc.narg(user_agent)
)
returning *;

-- name: GetSessionByTokenHash :one
select * from sessions where token_hash = sqlc.arg(token_hash);

-- name: RevokeSession :exec
update sessions set revoked_at = now() where token_hash = sqlc.arg(token_hash);

-- name: TouchSessionLastSeen :exec
update sessions set last_seen_at = now() where id = sqlc.arg(id);
