-- +migrate Up
create unique index outbound_wallets_unique on wallets (type, blockchain) where type = 'outbound';

-- +migrate Down
drop index outbound_wallets_unique;
