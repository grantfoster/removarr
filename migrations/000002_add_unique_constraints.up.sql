-- Add unique indexes for sonarr_id and radarr_id
-- This allows ON CONFLICT to work properly in sync operations
-- Using partial unique indexes to allow NULL values

CREATE UNIQUE INDEX IF NOT EXISTS unique_sonarr_id ON media_items(sonarr_id) 
WHERE sonarr_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS unique_radarr_id ON media_items(radarr_id) 
WHERE radarr_id IS NOT NULL;

