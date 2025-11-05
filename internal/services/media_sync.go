package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"removarr/internal/integrations"
)

type MediaSyncService struct {
	db           *sql.DB
	integrations *integrations.Client
}

type MediaItem struct {
	ID                 int
	Title              string
	Type               string // "movie" or "series"
	TMDBID             *int
	TVDBID             *int
	SonarrID           *int
	RadarrID           *int
	OverseerrRequestID *int
	RequestedByUserID  *int
	FilePath           string
	FileSize           int64
	AddedDate          *time.Time
}

func NewMediaSyncService(db *sql.DB, integrationsClient *integrations.Client) *MediaSyncService {
	return &MediaSyncService{
		db:           db,
		integrations: integrationsClient,
	}
}

// SyncFromSonarr fetches series from Sonarr and updates the database
func (s *MediaSyncService) SyncFromSonarr(ctx context.Context) error {
	if s.integrations.Sonarr == nil {
		return fmt.Errorf("sonarr integration not enabled")
	}

	slog.Info("Syncing media from Sonarr...")
	series, err := s.integrations.Sonarr.GetSeries()
	if err != nil {
		return fmt.Errorf("failed to fetch series from Sonarr: %w", err)
	}

	for _, ser := range series {
		size := int64(0)
		if ser.Statistics != nil {
			size = ser.Statistics.SizeOnDisk
		}

		addedDate, _ := time.Parse(time.RFC3339, ser.Added)

		// Series is downloaded if it has files (size > 0 and path exists)
		// Note: We still sync all series, even if not downloaded (monitored but not yet available)

		// Check if media item exists
		var existingID int
		err := s.db.QueryRowContext(ctx,
			"SELECT id FROM media_items WHERE sonarr_id = $1",
			ser.ID,
		).Scan(&existingID)

		if err == sql.ErrNoRows {
			// Insert new media item
			_, err = s.db.ExecContext(ctx,
				`INSERT INTO media_items 
					(title, type, sonarr_id, tvdb_id, file_path, file_size, added_date, last_synced_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP)
				ON CONFLICT (sonarr_id) WHERE sonarr_id IS NOT NULL DO UPDATE SET
					title = EXCLUDED.title,
					file_path = EXCLUDED.file_path,
					file_size = EXCLUDED.file_size,
					last_synced_at = CURRENT_TIMESTAMP`,
				ser.Title,
				"series",
				ser.ID,
				ser.TVDBID,
				ser.Path,
				size,
				addedDate,
			)
			if err != nil {
				slog.Error("Failed to insert media item", "error", err, "title", ser.Title)
				continue
			}
		} else if err == nil {
			// Update existing media item
			// Note: We preserve overseerr_request_id and requested_by_user_id if they exist
			_, err = s.db.ExecContext(ctx,
				`UPDATE media_items SET
					title = $2,
					file_path = $3,
					file_size = $4,
					last_synced_at = CURRENT_TIMESTAMP
				WHERE id = $1`,
				existingID,
				ser.Title,
				ser.Path,
				size,
			)
			if err != nil {
				slog.Error("Failed to update media item", "error", err, "id", existingID)
				continue
			}
		}
	}

	slog.Info("Sonarr sync complete", "count", len(series))
	return nil
}

// SyncFromRadarr fetches movies from Radarr and updates the database
func (s *MediaSyncService) SyncFromRadarr(ctx context.Context) error {
	if s.integrations.Radarr == nil {
		return fmt.Errorf("radarr integration not enabled")
	}

	slog.Info("Syncing media from Radarr...")
	movies, err := s.integrations.Radarr.GetMovies()
	if err != nil {
		return fmt.Errorf("failed to fetch movies from Radarr: %w", err)
	}

	for _, movie := range movies {
		size := int64(0)
		if movie.Statistics != nil {
			size = movie.Statistics.SizeOnDisk
		}

		addedDate, _ := time.Parse(time.RFC3339, movie.Added)

		// Note: We sync ALL movies from Radarr, including monitored but not yet downloaded
		// The "downloaded" status is determined in the API response based on file_size and file_path

		// Check if media item exists
		var existingID int
		err := s.db.QueryRowContext(ctx,
			"SELECT id FROM media_items WHERE radarr_id = $1",
			movie.ID,
		).Scan(&existingID)

		if err == sql.ErrNoRows {
			// Insert new media item (even if not downloaded - we track all monitored media)
			// Use INSERT ... ON CONFLICT with the unique index
			_, err = s.db.ExecContext(ctx,
				`INSERT INTO media_items 
					(title, type, radarr_id, tmdb_id, file_path, file_size, added_date, last_synced_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP)
				ON CONFLICT (radarr_id) WHERE radarr_id IS NOT NULL DO UPDATE SET
					title = EXCLUDED.title,
					file_path = EXCLUDED.file_path,
					file_size = EXCLUDED.file_size,
					last_synced_at = CURRENT_TIMESTAMP`,
				movie.Title,
				"movie",
				movie.ID,
				movie.TMDBID,
				movie.Path,
				size,
				addedDate,
			)
			if err != nil {
				slog.Error("Failed to insert media item", "error", err, "title", movie.Title)
				continue
			}
		} else if err == nil {
			// Update existing media item
			// Note: We preserve overseerr_request_id and requested_by_user_id if they exist
			_, err = s.db.ExecContext(ctx,
				`UPDATE media_items SET
					title = $2,
					file_path = $3,
					file_size = $4,
					last_synced_at = CURRENT_TIMESTAMP
				WHERE id = $1`,
				existingID,
				movie.Title,
				movie.Path,
				size,
			)
			if err != nil {
				slog.Error("Failed to update media item", "error", err, "id", existingID)
				continue
			}
		}
	}

	slog.Info("Radarr sync complete", "count", len(movies))
	return nil
}

// SyncOverseerrRequests links Overseerr requests to existing media items
func (s *MediaSyncService) SyncOverseerrRequests(ctx context.Context) error {
	if s.integrations.Overseerr == nil {
		return nil // Overseerr not enabled, skip
	}

	slog.Info("Syncing Overseerr requests...")
	requests, err := s.integrations.Overseerr.GetRequests()
	if err != nil {
		return fmt.Errorf("failed to fetch Overseerr requests: %w", err)
	}

	linkedCount := 0
	for _, req := range requests {
		// Determine media type (Overseerr uses "movie" or "tv")
		mediaType := req.MediaType
		if mediaType == "" {
			mediaType = req.Media.MediaType
		}
		if mediaType == "tv" {
			mediaType = "series" // Convert to our internal type
		}

		// Find matching media item by TMDB ID (movies) or TVDB ID (series)
		var mediaItemID int
		var queryErr error

		if mediaType == "movie" && req.Media.TMDBID > 0 {
			queryErr = s.db.QueryRowContext(ctx,
				"SELECT id FROM media_items WHERE tmdb_id = $1 AND type = 'movie'",
				req.Media.TMDBID,
			).Scan(&mediaItemID)
		} else if mediaType == "series" && req.Media.TVDBID != nil && *req.Media.TVDBID > 0 {
			queryErr = s.db.QueryRowContext(ctx,
				"SELECT id FROM media_items WHERE tvdb_id = $1 AND type = 'series'",
				*req.Media.TVDBID,
			).Scan(&mediaItemID)
		}

		if queryErr == sql.ErrNoRows {
			// No matching media item found, skip
			continue
		}
		if queryErr != nil {
			slog.Warn("Failed to find media item for Overseerr request",
				"request_id", req.ID,
				"tmdb_id", req.Media.TMDBID,
				"tvdb_id", req.Media.TVDBID,
				"error", queryErr)
			continue
		}

		// Update the media item with Overseerr request info
		_, err = s.db.ExecContext(ctx,
			`UPDATE media_items SET
				overseerr_request_id = $1,
				requested_by_user_id = $2,
				last_synced_at = CURRENT_TIMESTAMP
			WHERE id = $3`,
			req.ID,
			req.RequestedBy.ID,
			mediaItemID,
		)
		if err != nil {
			slog.Error("Failed to update media item with Overseerr request",
				"error", err,
				"media_item_id", mediaItemID,
				"request_id", req.ID)
			continue
		}

		linkedCount++
		slog.Debug("Linked Overseerr request to media item",
			"request_id", req.ID,
			"media_item_id", mediaItemID,
			"title", req.Media.Title)
	}

	slog.Info("Overseerr request sync complete", "linked", linkedCount, "total_requests", len(requests))
	return nil
}

// SyncAll syncs media from all enabled services
func (s *MediaSyncService) SyncAll(ctx context.Context) error {
	if s.integrations.Sonarr != nil {
		if err := s.SyncFromSonarr(ctx); err != nil {
			slog.Error("Sonarr sync failed", "error", err)
		}
	}

	if s.integrations.Radarr != nil {
		if err := s.SyncFromRadarr(ctx); err != nil {
			slog.Error("Radarr sync failed", "error", err)
		}
	}

	// Link Overseerr requests after syncing from Radarr/Sonarr
	// This ensures media items exist before we try to link requests
	if s.integrations.Overseerr != nil {
		if err := s.SyncOverseerrRequests(ctx); err != nil {
			slog.Error("Overseerr request sync failed", "error", err)
		}
	}

	return nil
}
