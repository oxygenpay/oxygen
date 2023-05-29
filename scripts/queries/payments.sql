-- name: GetPaymentByMerchantIDAndOrderUUID :one
SELECT * FROM payments
WHERE merchant_id = $1 and merchant_order_uuid = $2
LIMIT 1;

-- name: GetPaymentByID :one
SELECT * FROM payments WHERE id = $1
and (CASE WHEN @filter_by_merchant_id::boolean THEN merchant_id = $2 ELSE true END)
limit 1;

-- name: GetPaymentByPublicID :one
SELECT * FROM payments
WHERE public_id = $1
LIMIT 1;

-- name: GetPaymentByMerchantIDs :one
SELECT * FROM payments WHERE merchant_id = $1 and merchant_order_uuid = $2
LIMIT 1;

-- name: PaginatePaymentsAsc :many
SELECT * from payments
WHERE merchant_id = $1 and id >= $2
AND (CASE WHEN @filter_by_types::boolean THEN type = any(sqlc.arg(type)::varchar[]) ELSE true END)
ORDER BY id LIMIT $3;

-- name: PaginatePaymentsDesc :many
SELECT * from payments
WHERE merchant_id = $1 and id <= $2
AND (CASE WHEN @filter_by_types::boolean THEN type = any(sqlc.arg(type)::varchar[]) ELSE true END)
ORDER BY id desc LIMIT $3;

-- name: GetPaymentsByType :many
SELECT * from payments
where type = $1 and status = $2
and (CASE WHEN @filter_by_ids::boolean THEN id = any(sqlc.arg(id)::int[]) ELSE true END)
order by id limit $3;

-- name: GetBatchExpiredPayments :many
SELECT * from payments
where (
  (expires_at is not null and expires_at < $2)
  or (expires_at is null and created_at < $3)
)
and type = $4
and status = any(sqlc.arg(status)::varchar[])
order by id limit $1;

-- name: CreatePayment :one
INSERT INTO payments (
public_id,
created_at,
updated_at,
type,
status,
merchant_id,
merchant_order_uuid,
merchant_order_id,
expires_at,
price,
decimals,
currency,
description,
redirect_url,
metadata,
is_test
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: UpdatePaymentCustomerID :exec
UPDATE payments set customer_id = $1 where id = $2;

-- name: UpdatePayment :one
UPDATE payments
set status = $3,
updated_at = $4,
expires_at = (CASE WHEN @set_expires_at::boolean THEN $5 ELSE payments.expires_at END)
WHERE id = $1 and merchant_id = $2
returning *;

-- name: UpdatePaymentWebhookInfo :exec
UPDATE payments set webhook_sent_at = $3, updated_at = $4
WHERE id = $1 and merchant_id = $2;
