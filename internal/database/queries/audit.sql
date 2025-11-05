-- name: CreateAuditLog :one
INSERT INTO audit_logs (
    user_id, action, media_item_id, media_title, media_type, details
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListAuditLogsByUser :many
SELECT * FROM audit_logs
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

