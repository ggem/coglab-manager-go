-- name: CreateNote :one
insert into notes (entity_type, entity_id, author_user_id, body)
values (
    sqlc.arg(entity_type),
    sqlc.arg(entity_id),
    sqlc.arg(author_user_id),
    sqlc.arg(body)
)
returning *;

-- name: ListNotesByEntity :many
select * from notes
where entity_type = sqlc.arg(entity_type) and entity_id = sqlc.arg(entity_id)
order by created_at;
