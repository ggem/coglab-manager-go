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
