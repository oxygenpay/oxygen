-- +migrate Up
create table if not exists transactions
(
    id               bigserial constraint transactions_pkey primary key,
    created_at       timestamp      not null,
    updated_at       timestamp      not null,
    merchant_id      bigint         not null,

    status           varchar(16)    not null, -- "pending / completed / ...."

    type             varchar(16)    not null, -- "incoming / internal / withdrawal / ..."
    entity_id        bigint         null,     -- related entity (e.g. payment)

    wallet_id        bigint         not null,
    from_address     varchar(128)   null,
    to_address       varchar(128)   not null,
    transaction_hash varchar(128)   null,

    network          varchar(6)     not null, -- ETH, BTC, ...
    currency_type    varchar(16)    not null, -- COIN, TOKEN, ...
    currency         varchar(16)    not null, -- ETH, USDT, MATIC, ...
    decimals         int            not null, -- ETH has 18 dec.

    amount           decimal(64, 0) not null,
    fact_amount      decimal(64, 0) null,
    network_fee      decimal(64, 0) null,
    service_fee      decimal(64, 0) not null, -- service fee is included in the amount

    usd_amount       decimal(64, 0) not null, -- decimal is overkill but for consistency it's better to have the same types.

    metadata         jsonb          null
);

create index transactions_merchant_id on transactions (merchant_id);
create index transactions_entity_id on transactions (entity_id);
create index transactions_wallet_id on transactions (wallet_id);
create index transactions_from_address on transactions (from_address);
create index transactions_to_address on transactions (to_address);

-- +migrate Down
drop table if exists transactions;
