-- +migrate Up
alter table wallets
    add column confirmed_transactions bigint not null default 0 check (confirmed_transactions >= 0),
    add column pending_transactions   bigint not null default 0 check (pending_transactions >= 0);

-- +migrate Down
alter table wallets
    drop column confirmed_transactions,
    drop column pending_transactions;
