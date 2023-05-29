-- name: GetCustomerByID :one
SELECT * FROM customers
WHERE id = $1 and merchant_id = $2
LIMIT 1;

-- name: GetCustomerByUUID :one
SELECT * FROM customers
WHERE uuid = $1 and merchant_id = $2
LIMIT 1;

-- name: GetCustomerByEmail :one
SELECT * FROM customers
WHERE email = $1 and merchant_id = $2
LIMIT 1;

-- name: GetBatchCustomers :many
select * from customers where merchant_id = $1 and id =  any(sqlc.arg(ids)::int[]);

-- name: PaginateCustomersAsc :many
SELECT * from customers
WHERE merchant_id = $1 and id >= $2
ORDER BY id LIMIT $3;

-- name: PaginateCustomersDesc :many
SELECT * from customers
WHERE merchant_id = $1 and id <= $2
ORDER BY id desc LIMIT $3;

-- name: CalculateCustomerPayments :one
select count(id) from payments
where merchant_id = $1 and customer_id = $2 and status = $3
group by merchant_id, customer_id, status;

-- name: GetRecentCustomerPayments :many
select * from payments
where merchant_id = $1 and customer_id = $2
order by id desc limit $3;

-- name: CreateCustomer :one
INSERT INTO customers (
uuid,
created_at,
updated_at,
email,
merchant_id
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;