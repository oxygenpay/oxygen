-- +migrate Up
create table if not exists wallet_locks
(
    id           bigserial constraint wallet_locks_pkey primary key,
    wallet_id    bigint      not null,
    merchant_id  bigint      not null,
    currency     varchar(16) not null,
    network_id   varchar(16) not null,
    locked_at    timestamp   not null,
    locked_until timestamp   null,

    CONSTRAINT wallet_lock_is_unique UNIQUE (wallet_id, currency, network_id)
);

create index wallet_locks_wallet_id on wallet_locks (wallet_id);
create index wallet_locks_merchant_id on wallet_locks (merchant_id);

-- +migrate Down
drop index if exists wallet_locks_wallet_id;
drop index if exists wallet_locks_merchant_id;
drop table if exists wallet_locks;
