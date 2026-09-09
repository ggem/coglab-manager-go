-- +goose Up
-- Links a users row to its identity at an OIDC provider: the stable "sub"
-- claim, scoped by issuer -- not email (which can change) and not sub
-- alone (an institution that migrates IdPs over the years, e.g. changing
-- identity providers, could plausibly end up with two different providers
-- issuing the same sub value to two different people; scoping by issuer
-- as well makes that collision structurally impossible rather than
-- merely unlikely). Nullable and partially indexed, same pattern as
-- is_sitter_role/is_greeter_role: most rows are local-password-only
-- accounts with no SSO identity at all.
alter table users add column sso_issuer text;
alter table users add column sso_subject text;

create unique index users_sso_issuer_subject_key
    on users (sso_issuer, sso_subject)
    where sso_issuer is not null and sso_subject is not null;

-- +goose Down
drop index users_sso_issuer_subject_key;
alter table users drop column sso_subject;
alter table users drop column sso_issuer;
