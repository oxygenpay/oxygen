-- +migrate Up
alter table balances add column uuid uuid;
create index balances_uuid on balances (uuid);

-- +migrate Down
drop index if exists balances_uuid;
alter table balances drop column uuid;
