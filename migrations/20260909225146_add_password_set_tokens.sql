-- +goose Up
-- Backs the emailed "set your password" link for accounts created by a
-- platform or lab admin (see users.password_hash being nullable): the
-- account exists but can't log in locally until this link is redeemed.
-- Same shape as sessions -- token_hash, not the raw token, is all that's
-- ever stored -- but with used_at instead of revoked_at, since a
-- password-set token is meant to be single-use rather than revocable
-- ahead of time. Expiry and single-use are both enforced at query time,
-- not via an index predicate (a `where expires_at > now()` predicate
-- isn't immutable, so Postgres won't allow it as an index condition).
create table password_set_tokens (
    id          bigint generated always as identity primary key,
    token_hash  bytea not null unique,
    user_id     bigint not null references users(id),
    created_at  timestamptz not null default now(),
    expires_at  timestamptz not null,
    used_at     timestamptz
);

create index password_set_tokens_user_id_idx on password_set_tokens (user_id);

-- +goose Down
drop table password_set_tokens;
