-- +migrate Up
alter table payments
    add column webhook_sent_at timestamp null;

-- +migrate Down
alter table payments
    drop column webhook_sent_at;
