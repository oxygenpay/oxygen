-- +migrate Up
create table if not exists payment_links
(
  id              bigserial constraint payment_links_pkey primary key,
  uuid            uuid                  not null,
  slug            varchar(16)           not null,

  created_at      timestamp             not null,
  updated_at      timestamp             not null,

  merchant_id     bigint                not null,

  name            varchar(128)          not null,
  description     text                  not null,

  price           numeric(64)           not null,
  decimals        integer               not null,
  currency        varchar(16)           not null,

  success_action  varchar(16)           not null,
  redirect_url    text,
  success_message text,

  is_test         boolean default false not null
);

create index payments_links_uuid on payment_links (uuid);
create index payment_links_merchant_id on payment_links (merchant_id);
create unique index payment_links_slug on payment_links (slug);

-- +migrate Down
drop table if exists payment_links;
