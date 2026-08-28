-- +goose Up
alter table experiment_roles add column is_sitter_role boolean not null default false;

-- At most one role per lab can be designated the sitter role -- this is
-- an explicit, renameable per-lab designation (not a hardcoded ID or a
-- reserved name).  Used by the scheduling engine to decide whether a
-- sitter is available for an appointment.
create unique index experiment_roles_one_sitter_per_lab
    on experiment_roles (lab_id)
    where is_sitter_role;

-- +goose Down
drop index experiment_roles_one_sitter_per_lab;
alter table experiment_roles drop column is_sitter_role;
