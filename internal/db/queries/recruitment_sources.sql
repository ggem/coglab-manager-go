-- name: ListActiveRecruitmentSources :many
select * from recruitment_sources where active order by name;

-- name: CreateRecruitmentSource :one
insert into recruitment_sources (name) values (sqlc.arg(name))
returning *;
