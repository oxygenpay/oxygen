-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByGoogleID :one
SELECT * FROM users
WHERE google_id = $1 LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY id desc;

-- name: CreateUser :one
INSERT INTO users (
    name,
    email,
    uuid,
    google_id,
    profile_image_url,
    created_at,
    updated_at,
    deleted_at,
    settings
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET name = $1,
    profile_image_url= $2,
    updated_at = $3
WHERE id = $4
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;