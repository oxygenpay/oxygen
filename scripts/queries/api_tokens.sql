-- name: GetAPIToken :one
SELECT * FROM api_tokens
WHERE entity_type = $1 and token = $2 LIMIT 1;

-- name: GetAPITokenByUUID :one
SELECT * FROM api_tokens
WHERE uuid = $1 LIMIT 1;

-- name: ListAPITokensByEntity :many
SELECT * FROM api_tokens
WHERE entity_id = $1 and entity_type = $2
ORDER BY id DESC;

-- name: CreateAPIToken :one
INSERT INTO api_tokens (
    entity_type,
    entity_id,
    created_at,
    token,
    uuid,
    name,
    settings
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: DeleteAPITokenByToken :exec
DELETE from api_tokens
WHERE token = $1;

-- name: DeleteAPITokenByID :exec
DELETE from api_tokens
WHERE id = $1;