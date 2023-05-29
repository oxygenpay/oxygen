-- +migrate Up
create table if not exists customers
(
    id          bigserial constraint customers_pkey primary key,
    uuid        uuid unique  not null,
    created_at  timestamp    not null,
    updated_at  timestamp    not null,
    email       varchar(255) null,
    merchant_id bigint       not null
);

create index customers_merchant_id_index on customers (merchant_id);
create index customers_email_index on customers (email);

-- +migrate Down
drop table if exists customers;
