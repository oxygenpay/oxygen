-- +migrate Up
alter table wallets
    add column tatum_mainnet_subscription_id varchar(32) null,
    add column tatum_testnet_subscription_id varchar(32) null;

-- +migrate Down
alter table wallets
    drop column tatum_mainnet_subscription_id,
    drop column tatum_testnet_subscription_id;
