-- name: ListEligibleChildrenForExperiment :many
-- Every currently-active, not-already-held child eligible for a given
-- experiment within a scheduling window [window_start, window_end]:
-- their birth date must place their age inside the experiment's age
-- range at *some* point in the window, not necessarily today. Mirrors
-- legacy's hold-children formula: min_birth_date = window_start -
-- max_age, max_birth_date = window_end - min_age -- the widest birth-date
-- range for which some date in the window puts the child's age inside
-- [min_age, max_age]. Age-range and filter columns come straight from
-- the joined experiments row rather than being computed in Go first.
--
-- "Held" is derived, not a stored flag: not exists a live
-- (to_be_scheduled/pending) appointment for this child, for *any*
-- experiment -- enforced elsewhere by appointments_one_active_hold_per_
-- child, a partial unique index, not re-derived here as anything more
-- than a filter.
--
-- No ORDER BY/LIMIT: sort mode (oldest-first vs. random) and the
-- requested count are applied by the caller in Go.
select children.*
from children
join experiments on experiments.id = sqlc.arg(experiment_id)
where
    children.deactivated_at is null
    and children.birth_date is not null
    and children.birth_date >= sqlc.arg(window_start)::date - (experiments.age_range_max_months::float8 * interval '1 month')
    and children.birth_date <= sqlc.arg(window_end)::date - (experiments.age_range_min_months::float8 * interval '1 month')
    and (not experiments.filter_premies or coalesce(children.premie, false) = false)
    and (experiments.filter_min_languages = 0
         or coalesce(array_length(children.languages, 1), 0) >= experiments.filter_min_languages)
    and (cardinality(experiments.filter_languages) = 0
         or children.languages @> experiments.filter_languages)
    and (sqlc.narg(sex)::text is null or children.sex = sqlc.narg(sex))
    and not exists (
        select 1 from appointments
        where appointments.child_id = children.id
          and appointments.status in ('to_be_scheduled', 'pending')
    );
