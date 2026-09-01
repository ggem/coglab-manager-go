-- +goose Up

-- Legacy pairs the birth_comp flag with a free-text detail field
-- (bc_notes) that had no home in this schema until now.
alter table children add column birth_complications_notes text not null default '';

-- Guardian deletion was a hard delete, unlike Child's timestamp-based
-- soft-delete -- an inconsistency, not a deliberate choice. Bringing it
-- in line: a guardian a family withdrew shouldn't vanish from history.
alter table guardians add column deactivated_at timestamptz;

-- +goose Down
alter table guardians drop column deactivated_at;
alter table children drop column birth_complications_notes;
