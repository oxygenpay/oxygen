-- +migrate Up
alter table payments add column metadata jsonb null;

-- +migrate Down
alter table payments drop column metadata;

