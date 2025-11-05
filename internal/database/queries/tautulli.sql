-- name: GetTautulliHistory :one
SELECT * FROM tautulli_history
WHERE media_item_id = $1 AND user_id = $2 LIMIT 1;

-- name: GetTautulliHistoryByMediaItem :many
SELECT * FROM tautulli_history
WHERE media_item_id = $1;

-- name: CreateTautulliHistory :one
INSERT INTO tautulli_history (
    media_item_id, user_id, last_watched_at, play_count
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: UpdateTautulliHistory :one
UPDATE tautulli_history
SET 
    last_watched_at = $3,
    play_count = $4,
    last_synced_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE media_item_id = $1 AND user_id = $2
RETURNING *;

