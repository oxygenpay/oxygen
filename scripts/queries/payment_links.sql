-- name: ListPaymentLinks :many
select * from payment_links where merchant_id = $1 order by id desc limit $2;

-- name: GetPaymentLinkByID :one
select * from payment_links where merchant_id = $1 and id = $2 limit 1;

-- name: GetPaymentLinkBySlug :one
select * from payment_links where slug = $1 limit 1;

-- name: GetPaymentLinkByPublicID :one
select * from payment_links where merchant_id = $1 and uuid = $2 limit 1;

-- name: CreatePaymentLink :one
INSERT INTO payment_links (
  uuid,
  slug,
  created_at,
  updated_at,
  merchant_id,
  name,
  description,
  price,
  decimals,
  currency,
  success_action,
  redirect_url,
  success_message,
  is_test
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;


-- name: DeletePaymentLinkByPublicID :exec
delete from payment_links where merchant_id = $1 and uuid = $2;
