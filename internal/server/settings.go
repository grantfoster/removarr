package server

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// loadSettingsFromDB loads all settings from database and returns them as a map
func (s *Server) loadSettingsFromDB() (map[string]string, error) {
	settings := make(map[string]string)
	
	rows, err := s.db.Query("SELECT key, value FROM settings")
	if err != nil {
		if err == sql.ErrNoRows {
			return settings, nil
		}
		return nil, fmt.Errorf("failed to query settings: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan setting: %w", err)
		}
		settings[key] = value
	}
	
	return settings, rows.Err()
}

// getSetting gets a setting from database, returns defaultValue if not found
func (s *Server) getSetting(key, defaultValue string) string {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = $1", key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return defaultValue
		}
		slog.Warn("Failed to get setting", "key", key, "error", err)
		return defaultValue
	}
	return value
}

// setSetting sets a setting in the database
func (s *Server) setSetting(key, value, settingType string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value, type) 
		 VALUES ($1, $2, $3)
		 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = CURRENT_TIMESTAMP`,
		key, value, settingType,
	)
	return err
}

// loadIntegrationSettings loads integration settings from database and applies them to config
func (s *Server) loadIntegrationSettings() {
	// Load all settings at once
	dbSettings, err := s.loadSettingsFromDB()
	if err != nil {
		slog.Error("Failed to load settings from database, using config defaults", "error", err)
		return
	}
	
	// Helper to get setting with default
	getDBSetting := func(key, defaultValue string) string {
		if val, ok := dbSettings[key]; ok && val != "" {
			return val
		}
		return defaultValue
	}
	
	// Load integration settings
	// Overseerr
	if val := getDBSetting("overseerr.enabled", ""); val != "" {
		s.config.Overseerr.Enabled = val == "true"
	}
	if val := getDBSetting("overseerr.url", ""); val != "" {
		s.config.Overseerr.URL = val
	}
	if val := getDBSetting("overseerr.api_key", ""); val != "" {
		s.config.Overseerr.APIKey = val
	}
	
	// Sonarr
	if val := getDBSetting("sonarr.enabled", ""); val != "" {
		s.config.Sonarr.Enabled = val == "true"
	}
	if val := getDBSetting("sonarr.url", ""); val != "" {
		s.config.Sonarr.URL = val
	}
	if val := getDBSetting("sonarr.api_key", ""); val != "" {
		s.config.Sonarr.APIKey = val
	}
	
	// Radarr
	if val := getDBSetting("radarr.enabled", ""); val != "" {
		s.config.Radarr.Enabled = val == "true"
	}
	if val := getDBSetting("radarr.url", ""); val != "" {
		s.config.Radarr.URL = val
	}
	if val := getDBSetting("radarr.api_key", ""); val != "" {
		s.config.Radarr.APIKey = val
	}
	
	// Prowlarr
	if val := getDBSetting("prowlarr.enabled", ""); val != "" {
		s.config.Prowlarr.Enabled = val == "true"
	}
	if val := getDBSetting("prowlarr.url", ""); val != "" {
		s.config.Prowlarr.URL = val
	}
	if val := getDBSetting("prowlarr.api_key", ""); val != "" {
		s.config.Prowlarr.APIKey = val
	}
	
	// qBittorrent
	if val := getDBSetting("qbittorrent.enabled", ""); val != "" {
		s.config.QBittorrent.Enabled = val == "true"
	}
	if val := getDBSetting("qbittorrent.url", ""); val != "" {
		s.config.QBittorrent.URL = val
	}
	if val := getDBSetting("qbittorrent.username", ""); val != "" {
		s.config.QBittorrent.Username = val
	}
	if val := getDBSetting("qbittorrent.password", ""); val != "" {
		s.config.QBittorrent.Password = val
	}
	
	// Tautulli
	if val := getDBSetting("tautulli.enabled", ""); val != "" {
		s.config.Tautulli.Enabled = val == "true"
	}
	if val := getDBSetting("tautulli.url", ""); val != "" {
		s.config.Tautulli.URL = val
	}
	if val := getDBSetting("tautulli.api_key", ""); val != "" {
		s.config.Tautulli.APIKey = val
	}
	
	// Sync frequency
	if val := getDBSetting("sync_frequency", ""); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			s.config.Server.AutoSyncThreshold = duration
		}
	}
	
	slog.Info("Loaded integration settings from database")
}

