-- +migrate Up
create table if not exists payments
(
    id                  bigserial constraint payments_pkey primary key,
    public_id           uuid           not null,
    created_at          timestamp      not null,
    updated_at          timestamp      not null,
    type                varchar(16)    not null,
    status              varchar(16)    not null,


    merchant_id         bigint         not null,
    merchant_order_uuid uuid           not null,
    merchant_order_id   varchar(255)   null,

    expires_at          timestamp,

    price               decimal(64, 0) not null,
    decimals            int            not null,
    currency            varchar(16)    not null,

    description         text,
    redirect_url        text           not null,

    customer_id         bigint         null
);

create index payments_public_id on payments (public_id);
create index payments_merchant_id on payments (merchant_id);
create index payments_merchant_order_id on payments (merchant_order_id);
create index merchant_customer_id on payments (customer_id);
create unique index merchant_order_uuid on payments (merchant_id, merchant_order_uuid);

-- +migrate Down
drop table if exists payments;
