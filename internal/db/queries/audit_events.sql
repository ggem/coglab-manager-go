-- name: CreateAuditEvent :one
insert into audit_events (actor_user_id, lab_id, action, entity_type, entity_id, ip_address, metadata)
values (
    sqlc.narg(actor_user_id),
    sqlc.narg(lab_id),
    sqlc.arg(action),
    sqlc.narg(entity_type),
    sqlc.narg(entity_id),
    sqlc.narg(ip_address),
    sqlc.narg(metadata)
)
returning *;
