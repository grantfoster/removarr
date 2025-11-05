-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByPlexID :one
SELECT * FROM users
WHERE plex_id = $1 LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at DESC;

-- name: CreateUser :one
INSERT INTO users (
    username, email, password_hash, plex_id, plex_username, is_admin, is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET 
    username = $2,
    email = $3,
    password_hash = $4,
    plex_id = $5,
    plex_username = $6,
    is_admin = $7,
    is_active = $8,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

