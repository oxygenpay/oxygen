-- +migrate Up
alter table transactions
    rename column network to blockchain;

alter table transactions
    add column network_id varchar(16) null;

-- +migrate Down
alter table transactions
    rename column blockchain to network;

alter table transactions
    drop column network_id;
