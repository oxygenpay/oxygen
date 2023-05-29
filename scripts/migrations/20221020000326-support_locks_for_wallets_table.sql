-- +migrate Up
alter table wallets
    add column locked_for_merchant_id bigint    null,
    add column locked_at              timestamp null;

create index wallets_locked_for_merchant_id_index on wallets (locked_for_merchant_id);

-- +migrate Down
drop index wallets_locked_for_merchant_id_index;

alter table wallets
    drop column locked_for_merchant_id,
    drop column locked_at;
