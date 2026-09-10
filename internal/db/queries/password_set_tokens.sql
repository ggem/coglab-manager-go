-- name: CreatePasswordSetToken :one
insert into password_set_tokens (token_hash, user_id, expires_at)
values (sqlc.arg(token_hash), sqlc.arg(user_id), sqlc.arg(expires_at))
returning *;

-- name: ClaimPasswordSetToken :one
-- Atomically checks and consumes a token in one statement: the WHERE
-- clause and the used_at write happen under the same row lock, so two
-- concurrent redemptions of the same token can't both observe
-- used_at is null and both proceed -- the second one's WHERE clause
-- simply no longer matches once the first commits, returning no rows.
-- A separate SELECT-then-UPDATE (checking used_at, then writing it in a
-- later statement) would leave a window for exactly that race.
update password_set_tokens
set used_at = now()
where token_hash = sqlc.arg(token_hash) and used_at is null and expires_at > now()
returning *;
