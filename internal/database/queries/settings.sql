-- name: GetSetting :one
SELECT * FROM settings
WHERE key = $1 LIMIT 1;

-- name: ListSettings :many
SELECT * FROM settings
ORDER BY key;

-- name: CreateSetting :one
INSERT INTO settings (key, value, type)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateSetting :one
UPDATE settings
SET 
    value = $2,
    type = $3,
    updated_at = CURRENT_TIMESTAMP
WHERE key = $1
RETURNING *;

-- name: DeleteSetting :exec
DELETE FROM settings
WHERE key = $1;

