-- name: CreateTransaction :one
insert into transactions (
      merchant_id,
      created_at, updated_at,
      status, type, entity_id,
      sender_wallet_id, sender_address,
      recipient_wallet_id, recipient_address,
      transaction_hash,
      blockchain, network_id, currency_type, currency, decimals, network_decimals,
      amount, fact_amount, network_fee, service_fee, usd_amount,
      metadata, is_test
)
values (
      $1, $2, $3, $4, $5, $6,
      $7, $8, $9, $10, $11, $12,
      $13, $14, $15, $16, $17, $18,
      $19, $20, $21, $22, $23, $24
) returning *;

-- name: GetTransactionByID :one
select * from transactions where id = $1
and (CASE WHEN @filter_by_merchant_id::boolean THEN merchant_id = $2 ELSE true END);

-- name: GetTransactionByHashAndNetworkID :one
select * from transactions where transaction_hash = $1 and network_id = $2 limit 1;

-- name: GetLatestTransactionByPaymentID :one
select * from transactions where entity_id = $1 order by id desc limit 1;

-- name: EagerLoadTransactionsByPaymentID :many
select distinct on (entity_id) * from transactions
where merchant_id = $1 and entity_id = any(sqlc.arg(entity_ids)::int[])
and entity_id = any(sqlc.arg(entity_ids)::int[])
and type = any(sqlc.arg(type)::varchar[])
order by entity_id desc, id desc;

-- name: CancelTransaction :exec
update transactions
set status = $2,
updated_at = $3,
metadata = $4,
network_fee = CASE WHEN @set_network_fee::boolean THEN $5 ELSE transactions.network_fee END
where id = $1;

-- name: SetTransactionHash :exec
update transactions set transaction_hash = $1, updated_at = $2 where id = $3 and merchant_id = $4;

-- name: GetTransactionsByFilter :many
select * from transactions
where (CASE WHEN @filter_by_recipient_wallet_id::boolean THEN recipient_wallet_id = $1 ELSE true END)
and (CASE WHEN @filter_by_network_id::boolean THEN network_id = $2 ELSE true END)
and (CASE WHEN @filter_by_currency::boolean THEN currency = $3 ELSE true END)
and (CASE WHEN @filter_by_types::boolean THEN type = any(sqlc.arg(types)::varchar[]) ELSE true END)
and (CASE WHEN @filter_by_statuses::boolean THEN status = any(sqlc.arg(statuses)::varchar[]) ELSE true END)
and (CASE WHEN @filter_empty_hash::boolean THEN transaction_hash is null ELSE true END)
order by id desc
limit $4;

-- name: UpdateTransaction :one
update transactions set
status = $3,
updated_at = $4,
sender_address = $5,
fact_amount = $6,
transaction_hash = $7,
network_fee = $8,
service_fee = CASE WHEN @remove_service_fee::boolean THEN 0 ELSE transactions.service_fee END,
metadata = $9
where merchant_id = $1 and id = $2
returning *;