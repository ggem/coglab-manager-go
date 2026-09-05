-- +goose Up
alter table experiments
    add constraint experiments_sessions_check check (sessions >= 1),
    add constraint experiments_duration_minutes_check check (duration_minutes >= 1),
    add constraint experiments_filter_min_languages_check check (filter_min_languages >= 0),
    add constraint experiments_age_range_min_months_check check (age_range_min_months >= 0),
    add constraint experiments_age_range_max_months_check check (age_range_max_months >= 0);

-- +goose Down
alter table experiments
    drop constraint experiments_sessions_check,
    drop constraint experiments_duration_minutes_check,
    drop constraint experiments_filter_min_languages_check,
    drop constraint experiments_age_range_min_months_check,
    drop constraint experiments_age_range_max_months_check;
