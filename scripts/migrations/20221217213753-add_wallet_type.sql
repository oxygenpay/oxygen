-- +migrate Up
alter table wallets
    -- inbound / outbound / other
    add column type varchar(16) null;

-- +migrate Down
alter table wallets
    drop column type;
