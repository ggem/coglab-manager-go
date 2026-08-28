-- name: AddLabMemberTraining :exec
insert into lab_member_trainings (user_id, experiment_role_id)
values (sqlc.arg(user_id), sqlc.arg(experiment_role_id));

-- name: RemoveLabMemberTraining :exec
delete from lab_member_trainings
where user_id = sqlc.arg(user_id) and experiment_role_id = sqlc.arg(experiment_role_id);

-- name: ListLabMemberTrainingsForRole :many
-- The trained-member pool for one experiment_role -- the candidate list
-- the scheduling search's backtracking draws from for that role.
select users.* from users
join lab_member_trainings on lab_member_trainings.user_id = users.id
where lab_member_trainings.experiment_role_id = sqlc.arg(experiment_role_id)
  and users.deactivated_at is null
order by users.id;

-- name: ListLabMemberTrainingsForUser :many
select experiment_roles.* from experiment_roles
join lab_member_trainings on lab_member_trainings.experiment_role_id = experiment_roles.id
where lab_member_trainings.user_id = sqlc.arg(user_id)
order by experiment_roles.id;
