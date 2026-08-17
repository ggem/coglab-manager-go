-- name: CreateFamily :one
insert into families (address, city, state, zip, preferred_contact_method)
values (
    sqlc.arg(address),
    sqlc.arg(city),
    sqlc.arg(state),
    sqlc.arg(zip),
    sqlc.narg(preferred_contact_method)
)
returning *;

-- name: GetFamilyByID :one
select * from families where id = sqlc.arg(id);

-- name: UpdateFamily :one
update families set
    address = sqlc.arg(address),
    city = sqlc.arg(city),
    state = sqlc.arg(state),
    zip = sqlc.arg(zip),
    preferred_contact_method = sqlc.narg(preferred_contact_method)
where id = sqlc.arg(id)
returning *;

-- name: SearchFamilies :many
-- Existing-family lookup, primarily to catch duplicates before creating a
-- new family record for what's actually an existing one -- the legacy app
-- had this exact problem badly enough to need three separate hand-run
-- merge/dedupe SQL scripts. Matches if any guardian on the family matches
-- the name/email/phone filters, or the family's own address fields match.
-- A family with no guardians yet can't match on guardian filters (inner
-- join); that's fine since a family is always created together with its
-- first guardian in normal use.
--
-- Results are ordered by id rather than name-match quality: DISTINCT
-- requires ORDER BY expressions to be in the select list, and this is a
-- "review a handful of candidates" tool for staff, not a ranked search --
-- ranking isn't worth the extra query complexity here.
select distinct families.*
from families
join guardians on guardians.family_id = families.id
where
    (sqlc.narg(name_query)::text is null
        or word_similarity(sqlc.narg(name_query), guardians.first_name) > 0.2
        or word_similarity(sqlc.narg(name_query), guardians.last_name) > 0.2)
    and (sqlc.narg(email)::text is null or lower(guardians.email) = lower(sqlc.narg(email)))
    and (sqlc.narg(phone_number)::text is null or guardians.phone_number = sqlc.narg(phone_number))
    and (sqlc.narg(address)::text is null or families.address ilike '%' || sqlc.narg(address) || '%')
    and (sqlc.narg(city)::text is null or lower(families.city) = lower(sqlc.narg(city)))
    and (sqlc.narg(zip)::text is null or families.zip = sqlc.narg(zip))
order by families.id
limit sqlc.arg(limit_count);
