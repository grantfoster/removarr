-- name: GetTorrentByHash :one
SELECT * FROM torrents
WHERE hash = $1 LIMIT 1;

-- name: GetTorrentsByMediaItem :many
SELECT * FROM torrents
WHERE media_item_id = $1
ORDER BY created_at DESC;

-- name: ListTorrents :many
SELECT * FROM torrents
ORDER BY created_at DESC;

-- name: CreateTorrent :one
INSERT INTO torrents (
    media_item_id, hash, tracker_id, tracker_name, tracker_type,
    added_date, seeding_time_seconds, upload_bytes, download_bytes,
    ratio, seeding_required_seconds, seeding_required_ratio, is_seeding
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
RETURNING *;

-- name: UpdateTorrent :one
UPDATE torrents
SET 
    media_item_id = $2,
    tracker_id = $3,
    tracker_name = $4,
    tracker_type = $5,
    added_date = $6,
    seeding_time_seconds = $7,
    upload_bytes = $8,
    download_bytes = $9,
    ratio = $10,
    seeding_required_seconds = $11,
    seeding_required_ratio = $12,
    is_seeding = $13,
    last_synced_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE hash = $1
RETURNING *;

-- name: DeleteTorrent :exec
DELETE FROM torrents
WHERE hash = $1;

