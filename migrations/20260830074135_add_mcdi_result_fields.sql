-- +goose Up

-- Manual staff data-entry fields for MCDI/CDI survey results, matching
-- legacy's last_mcdi_pct/last_mcdi_date -- nothing in this app writes
-- these automatically (daxlabbase/cdibase has no result-webhook back
-- into this app; staff check results there directly and record them
-- here), same as legacy.
alter table children add column mcdi_percentile numeric(5, 2);
alter table children add column mcdi_date date;

-- +goose Down
alter table children drop column mcdi_date;
alter table children drop column mcdi_percentile;
