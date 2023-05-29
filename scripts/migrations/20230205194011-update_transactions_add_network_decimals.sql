-- +migrate Up
alter table transactions
    -- decimals of network itself (e.g. ETH), while 'decimals' column represents currency's digits (e.g. ETH_USDT)
    add column network_decimals int not null default 0;

-- +migrate Down
alter table transactions
    drop column network_decimals;
