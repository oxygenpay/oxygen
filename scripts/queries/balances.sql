-- name: ListBalances :many
select * from balances
where entity_type = $1 and entity_id = $2
order by id desc;

-- name: GetBalanceByUUID :one
select * from balances where entity_id = $1 and entity_type = $2 and uuid = $3;

-- name: GetBalanceByID :one
select * from balances where entity_id = $1 and entity_type = $2 and id = $3;

-- name: GetBalanceByIDWithLock :one
select * from balances where id = $1 FOR NO KEY UPDATE;

-- name: GetBalanceByFilter :one
select * from balances where entity_id = $1 and entity_type = $2 and network_id = $3 and currency = $4 limit 1;

-- name: GetBalanceByFilterWithLock :one
select * from balances
where entity_id = $1 and entity_type = $2 and network_id = $3 and currency = $4
FOR NO KEY UPDATE limit 1;

-- name: UpdateBalanceByID :one
update balances
set updated_at= $2, amount = balances.amount + $3
where id = $1
returning *;

-- name: CreateBalance :one
insert into balances (
  created_at, updated_at,
  entity_id, entity_type, uuid,
  network, network_id, currency_type, currency,
  decimals, amount
) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
returning *;

-- name: InsertBalanceAuditLog :exec
insert into balance_audit_log(created_at, balance_id, comment, metadata) values ($1,$2, $3, $4) returning *;

