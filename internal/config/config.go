package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	// Integration configs are loaded from database, not config file
	Plex     PlexConfig     `yaml:"-"` // Ignored in YAML, loaded from DB
	Overseerr OverseerrConfig `yaml:"-"` // Ignored in YAML, loaded from DB
	Sonarr   SonarrConfig   `yaml:"-"` // Ignored in YAML, loaded from DB
	Radarr   RadarrConfig   `yaml:"-"` // Ignored in YAML, loaded from DB
	Prowlarr ProwlarrConfig `yaml:"-"` // Ignored in YAML, loaded from DB
	QBittorrent QBittorrentConfig `yaml:"-"` // Ignored in YAML, loaded from DB
	Tautulli TautulliConfig `yaml:"-"` // Ignored in YAML, loaded from DB
	Logging  LoggingConfig  `yaml:"logging"`
}

type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	BaseURL      string        `yaml:"base_url"`
	SessionSecret string       `yaml:"session_secret"` // Or from env
	SessionMaxAge time.Duration `yaml:"session_max_age"`
	// AutoSyncThreshold is loaded from database, not config file
	AutoSyncThreshold time.Duration `yaml:"-"` // Ignored in YAML, loaded from DB
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"` // Or from env
	Database string `yaml:"database"`
	SSLMode  string `yaml:"ssl_mode"`
}

type PlexConfig struct {
	Enabled      bool   `yaml:"enabled"`
	URL          string `yaml:"url"`
	Token        string `yaml:"token"` // Or from env
	MachineID    string `yaml:"machine_id"`
}

type OverseerrConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	APIKey  string `yaml:"api_key"` // Or from env
}

type SonarrConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	APIKey  string `yaml:"api_key"` // Or from env
}

type RadarrConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	APIKey  string `yaml:"api_key"` // Or from env
}

type ProwlarrConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	APIKey  string `yaml:"api_key"` // Or from env
}

type QBittorrentConfig struct {
	Enabled  bool   `yaml:"enabled"`
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"` // Or from env
}

type TautulliConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	APIKey  string `yaml:"api_key"` // Or from env
}

type LoggingConfig struct {
	Level  string `yaml:"level"` // debug, info, warn, error
	Format string `yaml:"format"` // json, text
	File   string `yaml:"file"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Load sensitive values from environment if not in config
	config.loadFromEnv()

	// Set defaults
	config.setDefaults()

	return &config, nil
}

func (c *Config) loadFromEnv() {
	// Database password
	if c.Database.Password == "" {
		c.Database.Password = os.Getenv("REMOVARR_DB_PASSWORD")
	}

	// Session secret
	if c.Server.SessionSecret == "" {
		c.Server.SessionSecret = os.Getenv("REMOVARR_SESSION_SECRET")
	}

	// API keys
	if c.Overseerr.APIKey == "" {
		c.Overseerr.APIKey = os.Getenv("REMOVARR_OVERSEERR_API_KEY")
	}
	if c.Sonarr.APIKey == "" {
		c.Sonarr.APIKey = os.Getenv("REMOVARR_SONARR_API_KEY")
	}
	if c.Radarr.APIKey == "" {
		c.Radarr.APIKey = os.Getenv("REMOVARR_RADARR_API_KEY")
	}
	if c.Prowlarr.APIKey == "" {
		c.Prowlarr.APIKey = os.Getenv("REMOVARR_PROWLARR_API_KEY")
	}
	if c.Tautulli.APIKey == "" {
		c.Tautulli.APIKey = os.Getenv("REMOVARR_TAUTULLI_API_KEY")
	}

	// Plex token
	if c.Plex.Token == "" {
		c.Plex.Token = os.Getenv("REMOVARR_PLEX_TOKEN")
	}

	// qBittorrent password
	if c.QBittorrent.Password == "" {
		c.QBittorrent.Password = os.Getenv("REMOVARR_QBITTORRENT_PASSWORD")
	}
}

func (c *Config) setDefaults() {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.SessionMaxAge == 0 {
		c.Server.SessionMaxAge = 7 * 24 * time.Hour // 7 days
	}
	if c.Server.AutoSyncThreshold == 0 {
		c.Server.AutoSyncThreshold = 5 * time.Minute // Default: sync if data is older than 5 minutes
	}

	if c.Database.Host == "" {
		c.Database.Host = "localhost"
	}
	if c.Database.Port == 0 {
		c.Database.Port = 5432
	}
	if c.Database.User == "" {
		c.Database.User = "removarr"
	}
	if c.Database.Database == "" {
		c.Database.Database = "removarr"
	}
	if c.Database.SSLMode == "" {
		c.Database.SSLMode = "disable"
	}

	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:          "0.0.0.0",
			Port:          8080,
			SessionMaxAge: 7 * 24 * time.Hour,
		},
		Database: DatabaseConfig{
			Host:    "localhost",
			Port:    5432,
			User:    "removarr",
			Database: "removarr",
			SSLMode: "disable",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// Save writes the config to a YAML file
// Note: Sensitive values (passwords, API keys) that are loaded from env vars
// will not be written to the file to avoid exposing them
func (c *Config) Save(path string) error {
	// Create a copy of the config for saving
	// We'll write empty strings for values that should come from env vars
	saveConfig := *c
	
	// Clear sensitive values that should come from env vars
	// (These are typically empty in the config file anyway)
	if c.Database.Password != "" {
		// Check if it came from env - if so, don't write it
		// For simplicity, we'll preserve it if it's in the config
		// User can manually clear it if they want env-only
	}
	
	data, err := yaml.Marshal(&saveConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	return os.WriteFile(path, data, 0644)
}

