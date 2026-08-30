-- name: CreateZipCode :one
insert into zipcodes (lab_id, zip_code, priority) values (sqlc.arg(lab_id), sqlc.arg(zip_code), sqlc.arg(priority))
returning *;

-- name: GetZipCodeByID :one
select * from zipcodes where id = sqlc.arg(id);

-- name: ListZipCodesByLab :many
select * from zipcodes where lab_id = sqlc.arg(lab_id) order by zip_code;

-- name: UpdateZipCode :one
update zipcodes set zip_code = sqlc.arg(zip_code), priority = sqlc.arg(priority)
where id = sqlc.arg(id)
returning *;

-- name: DeactivateZipCode :exec
update zipcodes set deactivated_at = now() where id = sqlc.arg(id);
