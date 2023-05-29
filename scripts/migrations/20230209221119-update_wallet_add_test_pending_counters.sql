-- +migrate Up
alter table wallets rename column pending_transactions to pending_mainnet_transactions;
alter table wallets rename column confirmed_transactions to confirmed_mainnet_transactions;

alter table wallets add column pending_testnet_transactions bigint not null default 0;
alter table wallets add column confirmed_testnet_transactions bigint not null default 0;

-- +migrate Down
alter table wallets rename column pending_mainnet_transactions to pending_transactions;
alter table wallets rename column confirmed_mainnet_transactions to confirmed_transactions;

alter table wallets drop column pending_testnet_transactions;
alter table wallets drop column confirmed_testnet_transactions;

