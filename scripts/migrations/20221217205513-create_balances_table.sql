-- +migrate Up
create table if not exists balances
(
    id            bigserial constraint balances_pkey primary key,

    created_at    timestamp      not null,
    updated_at    timestamp      not null,

    entity_id     bigint         not null,
    entity_type   varchar(16)    not null,

    network       varchar(6)     not null, -- ETH, BTC, ...
    network_id    varchar(16)    not null, -- test / prod
    currency_type varchar(16)    not null, -- COIN, TOKEN, ...
    currency      varchar(16)    not null, -- ETH, USDT, MATIC, ...
    decimals      int            not null, -- ETH has 18 decimals

    amount        decimal(64, 0) not null check (amount > 0),

    CONSTRAINT balance_properties_unique UNIQUE (entity_type, entity_id, network, network_id, currency)
);

create table if not exists balance_audit_log
(
    id         bigserial constraint balance_audit_log_pkey primary key,
    created_at timestamp not null,
    balance_id bigint    not null,
    comment    text      not null,
    metadata   jsonb     null
);

create index balances_entity_index on balances (entity_type, entity_id);
create index balance_audit_log_balance_id_index on balance_audit_log (balance_id);

-- +migrate Down
drop table if exists balances;
drop table if exists balance_audit_log;
