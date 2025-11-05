-- name: GetMediaItemByID :one
SELECT * FROM media_items
WHERE id = $1 LIMIT 1;

-- name: GetMediaItemBySonarrID :one
SELECT * FROM media_items
WHERE sonarr_id = $1 LIMIT 1;

-- name: GetMediaItemByRadarrID :one
SELECT * FROM media_items
WHERE radarr_id = $1 LIMIT 1;

-- name: ListMediaItems :many
SELECT * FROM media_items
ORDER BY created_at DESC;

-- name: ListMediaItemsByUser :many
SELECT * FROM media_items
WHERE requested_by_user_id = $1
ORDER BY created_at DESC;

-- name: CreateMediaItem :one
INSERT INTO media_items (
    title, type, tmdb_id, tvdb_id, sonarr_id, radarr_id, 
    overseerr_request_id, requested_by_user_id, file_path, 
    file_size, added_date
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: UpdateMediaItem :one
UPDATE media_items
SET 
    title = $2,
    type = $3,
    tmdb_id = $4,
    tvdb_id = $5,
    sonarr_id = $6,
    radarr_id = $7,
    overseerr_request_id = $8,
    requested_by_user_id = $9,
    file_path = $10,
    file_size = $11,
    added_date = $12,
    last_synced_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteMediaItem :exec
DELETE FROM media_items
WHERE id = $1;

