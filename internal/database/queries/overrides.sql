-- name: GetSeedingOverrideByTrackerID :one
SELECT * FROM seeding_overrides
WHERE tracker_id = $1 LIMIT 1;

-- name: GetSeedingOverrideByTrackerName :one
SELECT * FROM seeding_overrides
WHERE tracker_name = $1 LIMIT 1;

-- name: ListSeedingOverrides :many
SELECT * FROM seeding_overrides
ORDER BY tracker_name;

-- name: CreateSeedingOverride :one
INSERT INTO seeding_overrides (
    tracker_id, tracker_name, min_seeding_time_seconds, min_seeding_ratio
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: UpdateSeedingOverride :one
UPDATE seeding_overrides
SET 
    tracker_id = $2,
    tracker_name = $3,
    min_seeding_time_seconds = $4,
    min_seeding_ratio = $5,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteSeedingOverride :exec
DELETE FROM seeding_overrides
WHERE id = $1;

