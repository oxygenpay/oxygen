-- name: GetWalletByID :one
SELECT *
FROM wallets
WHERE id = $1
LIMIT 1;

-- name: GetAvailableWallet :one
SELECT wallets.*
FROM wallets
WHERE blockchain = $1 and type = $2
AND NOT EXISTS(
    select id from wallet_locks
    where wallet_id = wallets.id and currency = $3 and network_id = $4
)
order by id
LIMIT 1;

-- name: GetWalletByUUID :one
SELECT *
FROM wallets
WHERE uuid = $1
LIMIT 1;

-- name: PaginateWalletsByID :many
SELECT *
FROM wallets
WHERE id >= $1
AND (CASE WHEN @filter_by_type::bool THEN type = $4 ELSE true END)
AND (CASE WHEN @filter_by_blockchain::bool THEN blockchain = $3 ELSE true END)
order by id
LIMIT $2;

-- name: CheckSystemWalletExistsByAddress :one
SELECT * from wallets where address = $1 limit 1;

-- name: CreateWallet :one
INSERT INTO wallets (
    created_at,
    uuid,
    address,
    blockchain,
    type
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateWalletTatumFields :one
UPDATE wallets
SET tatum_mainnet_subscription_id = $1,
    tatum_testnet_subscription_id = $2
WHERE id = $3
RETURNING *;

-- name: GetWalletForUpdateByID :one
SELECT * FROM wallets WHERE id = $1 LIMIT 1 FOR NO KEY UPDATE;

-- name: UpdateWalletMainnetTransactionCounters :exec
update wallets set confirmed_mainnet_transactions = $2, pending_mainnet_transactions = $3 where id = $1;

-- name: UpdateWalletTestnetTransactionCounters :exec
update wallets set confirmed_testnet_transactions = $2, pending_testnet_transactions = $3 where id = $1;

-- name: CreateWalletLock :one
INSERT INTO wallet_locks (
    merchant_id,
    wallet_id,
    currency,
    network_id,
    locked_at,
    locked_until
) VALUES ($1, $2, $3, $4, $5, $6)
returning *;

-- name: GetWalletLock :one
select * from wallet_locks where wallet_id = $1 and currency = $2 and network_id = $3 limit 1;

-- name: ReleaseWalletLock :exec
DELETE from wallet_locks where id = $1;