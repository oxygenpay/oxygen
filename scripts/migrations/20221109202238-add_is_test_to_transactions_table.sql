-- +migrate Up
alter table transactions add column is_test bool default false not null;

-- +migrate Down
alter table transactions drop column is_test;
