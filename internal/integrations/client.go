package integrations

import (
	"removarr/internal/config"
)

// Client manages all integration clients
type Client struct {
	Overseerr   *OverseerrClient
	Sonarr      *SonarrClient
	Radarr      *RadarrClient
	Prowlarr    *ProwlarrClient
	QBittorrent *QBittorrentClient
	Tautulli    *TautulliClient
	Plex        *PlexClient
}

func NewClient(cfg *config.Config) *Client {
	client := &Client{}

	if cfg.Overseerr.Enabled {
		client.Overseerr = NewOverseerrClient(cfg.Overseerr.URL, cfg.Overseerr.APIKey)
	}

	if cfg.Sonarr.Enabled {
		client.Sonarr = NewSonarrClient(cfg.Sonarr.URL, cfg.Sonarr.APIKey)
	}

	if cfg.Radarr.Enabled {
		client.Radarr = NewRadarrClient(cfg.Radarr.URL, cfg.Radarr.APIKey)
	}

	if cfg.Prowlarr.Enabled {
		client.Prowlarr = NewProwlarrClient(cfg.Prowlarr.URL, cfg.Prowlarr.APIKey)
	}

	if cfg.QBittorrent.Enabled {
		client.QBittorrent = NewQBittorrentClient(cfg.QBittorrent.URL, cfg.QBittorrent.Username, cfg.QBittorrent.Password)
	}

	if cfg.Tautulli.Enabled {
		client.Tautulli = NewTautulliClient(cfg.Tautulli.URL, cfg.Tautulli.APIKey)
	}

	if cfg.Plex.Enabled {
		client.Plex = NewPlexClient(cfg.Plex.URL, cfg.Plex.Token)
	}

	return client
}
