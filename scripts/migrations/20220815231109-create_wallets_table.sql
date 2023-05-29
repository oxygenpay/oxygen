-- +migrate Up
create table if not exists wallets
(
    id         bigserial constraint wallets_pkey primary key,
    created_at timestamp   not null,
    uuid       uuid unique not null,
    address    text        not null,
    blockchain varchar(6)  not null
);

create index wallets_blockchain_index on wallets (blockchain);

-- +migrate Down
drop table if exists wallets;
