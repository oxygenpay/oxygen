-- +migrate Up
create table if not exists merchant_addresses
(
    id          bigserial constraint merchant_addresses_pkey primary key,
    uuid        uuid         not null,

    created_at  timestamp    not null,
    updated_at  timestamp    not null,

    merchant_id bigint       not null,

    name        varchar(128) not null,
    blockchain  varchar(16)  not null,
    address     varchar(128) not null
);

create index merchant_addresses_merchant_uuid on merchant_addresses (uuid);
create index merchant_addresses_merchant_id on merchant_addresses (merchant_id);

-- +migrate Down
drop table if exists merchant_addresses;
