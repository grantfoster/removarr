-- Users table
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255),
    password_hash VARCHAR(255), -- NULL for Plex users
    plex_id INTEGER UNIQUE, -- Plex user ID if authenticated via Plex
    plex_username VARCHAR(255), -- Plex username
    is_admin BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Media items cache
CREATE TABLE media_items (
    id SERIAL PRIMARY KEY,
    title VARCHAR(500) NOT NULL,
    type VARCHAR(50) NOT NULL, -- 'movie' or 'series'
    tmdb_id INTEGER,
    tvdb_id INTEGER,
    sonarr_id INTEGER,
    radarr_id INTEGER,
    overseerr_request_id INTEGER,
    requested_by_user_id INTEGER REFERENCES users(id),
    file_path TEXT,
    file_size BIGINT, -- in bytes
    added_date TIMESTAMP,
    last_synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for media_items
CREATE INDEX idx_media_items_type ON media_items(type);
CREATE INDEX idx_media_items_sonarr_id ON media_items(sonarr_id);
CREATE INDEX idx_media_items_radarr_id ON media_items(radarr_id);
CREATE INDEX idx_media_items_requested_by ON media_items(requested_by_user_id);
CREATE INDEX idx_media_items_last_synced ON media_items(last_synced_at);

-- Torrents tracking
CREATE TABLE torrents (
    id SERIAL PRIMARY KEY,
    media_item_id INTEGER REFERENCES media_items(id) ON DELETE CASCADE,
    hash VARCHAR(64) NOT NULL UNIQUE, -- qBittorrent hash
    tracker_id INTEGER, -- Prowlarr tracker ID
    tracker_name VARCHAR(255),
    tracker_type VARCHAR(50), -- 'public' or 'private'
    added_date TIMESTAMP,
    seeding_time_seconds BIGINT DEFAULT 0,
    upload_bytes BIGINT DEFAULT 0,
    download_bytes BIGINT DEFAULT 0,
    ratio DECIMAL(10, 2) DEFAULT 0,
    seeding_required_seconds BIGINT, -- from Prowlarr or override
    seeding_required_ratio DECIMAL(10, 2), -- from Prowlarr or override
    is_seeding BOOLEAN DEFAULT TRUE,
    last_synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for torrents
CREATE INDEX idx_torrents_media_item ON torrents(media_item_id);
CREATE INDEX idx_torrents_hash ON torrents(hash);
CREATE INDEX idx_torrents_tracker ON torrents(tracker_id);

-- Seeding overrides (per-tracker custom requirements)
CREATE TABLE seeding_overrides (
    id SERIAL PRIMARY KEY,
    tracker_id INTEGER,
    tracker_name VARCHAR(255),
    min_seeding_time_seconds BIGINT,
    min_seeding_ratio DECIMAL(10, 2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tracker_id)
);

-- Settings (application configuration)
CREATE TABLE settings (
    key VARCHAR(255) PRIMARY KEY,
    value TEXT,
    type VARCHAR(50) DEFAULT 'string', -- 'string', 'integer', 'boolean', 'json'
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Tautulli watch history cache
CREATE TABLE tautulli_history (
    id SERIAL PRIMARY KEY,
    media_item_id INTEGER REFERENCES media_items(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id),
    last_watched_at TIMESTAMP,
    play_count INTEGER DEFAULT 0,
    last_synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(media_item_id, user_id)
);

-- Index for tautulli_history
CREATE INDEX idx_tautulli_history_media_item ON tautulli_history(media_item_id);
CREATE INDEX idx_tautulli_history_user ON tautulli_history(user_id);
CREATE INDEX idx_tautulli_history_last_watched ON tautulli_history(last_watched_at);

-- Audit log (deletions only)
CREATE TABLE audit_logs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    action VARCHAR(50) NOT NULL, -- 'delete'
    media_item_id INTEGER REFERENCES media_items(id) ON DELETE SET NULL,
    media_title VARCHAR(500),
    media_type VARCHAR(50),
    details JSONB, -- Additional context about the deletion
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for audit_logs
CREATE INDEX idx_audit_logs_user ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_media_item ON audit_logs(media_item_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);

