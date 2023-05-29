-- name: GetMerchantByID :one
SELECT * FROM merchants
WHERE id = $1
AND (CASE WHEN @with_trashed::boolean THEN true ELSE deleted_at is null END)
LIMIT 1;

-- name: GetMerchantByUUID :one
SELECT * FROM merchants
WHERE uuid = $1
AND (CASE WHEN @with_trashed::boolean THEN true ELSE deleted_at is null END)
LIMIT 1;

-- name: GetMerchantByUUIDAndCreatorID :one
SELECT * FROM merchants
WHERE uuid = $1 and creator_id = $2
AND (CASE WHEN @with_trashed::boolean THEN true ELSE deleted_at is null END)
LIMIT 1;

-- name: ListMerchantsByCreatorID :many
SELECT * FROM merchants
WHERE creator_id = $1
AND (CASE WHEN @with_trashed::boolean THEN true ELSE deleted_at is null END)
ORDER BY id desc;

-- name: UpdateMerchantSettings :exec
UPDATE merchants
SET updated_at = $2, settings = $3
WHERE id = $1;

-- name: SoftDeleteMerchantByUUID :exec
UPDATE merchants
SET deleted_at = current_timestamp
WHERE uuid = $1;

-- name: CreateMerchant :one
INSERT INTO merchants (
    uuid,
    created_at,
    updated_at,
    deleted_at,
    name,
    website,
    creator_id,
    settings
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateMerchant :one
UPDATE merchants
SET
    updated_at = $2,
    name = $3,
    website = $4
WHERE id = $1
RETURNING *;
