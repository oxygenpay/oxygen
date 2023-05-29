-- +migrate Up
alter table payments add column is_test bool default false not null;

-- +migrate Down
alter table payments drop column is_test;
