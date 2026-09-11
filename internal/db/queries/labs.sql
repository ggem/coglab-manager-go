-- name: ListLabs :many
-- Every lab in the system, regardless of membership -- unlike
-- ListLabsForUser (lab_memberships.sql), this is for the
-- platform-admin "assign a lab" picker, where the admin may not
-- belong to the lab they're assigning someone else to. labs has no
-- deactivated_at column, so there's nothing to filter out.
select * from labs order by name;
