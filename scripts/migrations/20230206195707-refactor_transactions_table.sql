-- +migrate Up
-- 1. Rename string addresses
alter table transactions rename column from_address to sender_address;
alter table transactions rename column to_address to recipient_address;

-- 2. Rename wallet & make it nullable
alter table transactions rename column wallet_id to recipient_wallet_id;
alter table transactions alter column recipient_wallet_id drop not null;

-- 3. Add sender wallet id
alter table transactions add column sender_wallet_id bigint null;

-- 4. Change indexes
alter index transactions_wallet_id rename to transactions_recipient_wallet_id;
alter index transactions_from_address rename to transactions_sender_address;
alter index transactions_to_address rename to transactions_recipient_address;

create index transactions_sender_wallet_id on transactions (sender_wallet_id);

-- +migrate Down
drop index if exists transactions_sender_wallet_id;

alter index transactions_recipient_wallet_id rename to transactions_wallet_id;
alter index transactions_sender_address rename to transactions_from_address;
alter index transactions_recipient_address rename to transactions_to_address;

alter table transactions drop column sender_wallet_id;

alter table transactions rename column recipient_wallet_id to wallet_id;
alter table transactions alter column wallet_id set not null;

alter table transactions rename column sender_address to from_address;
alter table transactions rename column recipient_address to to_address;

