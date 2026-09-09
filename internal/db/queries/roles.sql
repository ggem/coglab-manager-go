-- name: ListRoles :many
-- The fixed, small set of permission-level roles (staff/coordinator/
-- admin) a lab membership can be assigned -- not lab-scoped, and not
-- editable from the lab-members admin page (see lab_memberships.sql).
select * from roles order by id;
