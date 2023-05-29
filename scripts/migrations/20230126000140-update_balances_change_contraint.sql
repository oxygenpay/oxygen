-- +migrate Up
alter table balances drop constraint balances_amount_check;
alter table balances add constraint balances_amount_check check (amount >= 0);

-- +migrate Down
alter table balances drop constraint balances_amount_check;
alter table balances add constraint balances_amount_check check (amount > 0);
