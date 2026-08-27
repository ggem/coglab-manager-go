-- +goose Up
alter table equipment add constraint equipment_quantity_check check (quantity >= 0);

-- +goose Down
alter table equipment drop constraint equipment_quantity_check;
