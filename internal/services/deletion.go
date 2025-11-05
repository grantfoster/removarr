package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"removarr/internal/integrations"
)

type DeletionService struct {
	db          *sql.DB
	sonarr      *integrations.SonarrClient
	radarr      *integrations.RadarrClient
	overseerr   *integrations.OverseerrClient
	qbittorrent *integrations.QBittorrentClient
}

func NewDeletionService(
	db *sql.DB,
	sonarr *integrations.SonarrClient,
	radarr *integrations.RadarrClient,
	overseerr *integrations.OverseerrClient,
	qbittorrent *integrations.QBittorrentClient,
) *DeletionService {
	return &DeletionService{
		db:          db,
		sonarr:      sonarr,
		radarr:      radarr,
		overseerr:   overseerr,
		qbittorrent: qbittorrent,
	}
}

// DeleteMediaItem performs the complete deletion workflow:
// 1. Get media item from DB
// 2. Delete files from filesystem (if downloaded)
// 3. Delete/unmonitor from Sonarr/Radarr
// 4. Delete from Overseerr (if requested)
// 5. Delete torrent from qBittorrent
// 6. Log to audit log
// 7. Delete from database
func (s *DeletionService) DeleteMediaItem(ctx context.Context, mediaID int, userID int) error {
	// Step 1: Get media item from DB
	var (
		id                 int
		title              string
		mediaType          string
		sonarrID           sql.NullInt64
		radarrID           sql.NullInt64
		overseerrRequestID sql.NullInt64
		filePath           sql.NullString
		fileSize           sql.NullInt64
	)
	var tmdbID sql.NullInt64
	var tvdbID sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, type, sonarr_id, radarr_id, overseerr_request_id, file_path, file_size, tmdb_id, tvdb_id
		FROM media_items
		WHERE id = $1
	`, mediaID).Scan(
		&id, &title, &mediaType,
		&sonarrID, &radarrID, &overseerrRequestID,
		&filePath, &fileSize,
		&tmdbID, &tvdbID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("media item not found: %d", mediaID)
		}
		return fmt.Errorf("failed to get media item: %w", err)
	}

	slog.Info("Starting media deletion", "media_id", mediaID, "title", title, "type", mediaType)

	// Track errors but continue with deletion
	var errors []string

	// Step 2: Delete files from filesystem (if downloaded)
	// We delete files ourselves first to ensure they're removed from disk
	// This is a critical requirement - files MUST be deleted from disk
	if filePath.Valid && filePath.String != "" {
		if err := s.deleteFiles(filePath.String); err != nil {
			errors = append(errors, fmt.Sprintf("failed to delete files: %v", err))
			slog.Error("Failed to delete files", "path", filePath.String, "error", err)
		} else {
			slog.Info("Deleted files from disk", "path", filePath.String)
		}
	}

	// Step 3: Delete/unmonitor from Sonarr/Radarr
	// Note: We pass deleteFiles=false since we already deleted files ourselves
	// If our deletion failed, we could pass true, but Radarr/Sonarr might fail
	// if files don't exist, so we'll just unmonitor if delete fails
	if mediaType == "series" && sonarrID.Valid && s.sonarr != nil {
		// Try to delete from Sonarr (will unmonitor even if files already deleted)
		// addImportExclusion=false prevents the series from being added to the exclusion list
		if err := s.sonarr.DeleteSeries(int(sonarrID.Int64), false, false); err != nil {
			// If delete fails, try unmonitoring
			slog.Warn("Failed to delete from Sonarr, trying unmonitor", "error", err)
			if err := s.sonarr.UnmonitorSeries(int(sonarrID.Int64)); err != nil {
				errors = append(errors, fmt.Sprintf("failed to delete/unmonitor from Sonarr: %v", err))
				slog.Error("Failed to unmonitor from Sonarr", "error", err)
			} else {
				slog.Info("Unmonitored from Sonarr", "sonarr_id", sonarrID.Int64)
			}
		} else {
			slog.Info("Deleted from Sonarr (not added to exclusion list)", "sonarr_id", sonarrID.Int64)
		}
	} else if mediaType == "movie" && radarrID.Valid && s.radarr != nil {
		// Try to delete from Radarr first (this removes the movie entry completely)
		// Note: Radarr's DELETE endpoint removes the movie from its database
		// If deleteFiles=false, it won't delete files, but it WILL remove the movie entry
		// addImportExclusion=false prevents the movie from being added to the exclusion list
		if err := s.radarr.DeleteMovie(int(radarrID.Int64), false, false); err != nil {
			// If delete fails (e.g., movie not found, or API error), try unmonitoring as fallback
			slog.Warn("Failed to delete from Radarr, trying unmonitor as fallback", "error", err, "radarr_id", radarrID.Int64)
			if err := s.radarr.UnmonitorMovie(int(radarrID.Int64)); err != nil {
				errors = append(errors, fmt.Sprintf("failed to delete/unmonitor from Radarr: %v", err))
				slog.Error("Failed to unmonitor from Radarr", "error", err, "radarr_id", radarrID.Int64)
			} else {
				slog.Info("Successfully unmonitored movie in Radarr", "radarr_id", radarrID.Int64)
			}
		} else {
			slog.Info("Successfully deleted movie from Radarr (not added to exclusion list)", "radarr_id", radarrID.Int64)
		}
	}

	// Step 4: Delete from Overseerr (if requested)
	// If we don't have a request ID stored, try to find it by TMDB/TVDB ID
	if s.overseerr != nil {
		var requestID int
		if overseerrRequestID.Valid {
			requestID = int(overseerrRequestID.Int64)
			slog.Info("Using stored Overseerr request ID", "request_id", requestID)
		} else {
			// Try to find the request by TMDB/TVDB ID
			var tmdbIDPtr *int
			var tvdbIDPtr *int
			if tmdbID.Valid && tmdbID.Int64 > 0 {
				id := int(tmdbID.Int64)
				tmdbIDPtr = &id
			}
			if tvdbID.Valid && tvdbID.Int64 > 0 {
				id := int(tvdbID.Int64)
				tvdbIDPtr = &id
			}

			if tmdbIDPtr != nil || tvdbIDPtr != nil {
				req, err := s.overseerr.FindRequestByMediaID(tmdbIDPtr, tvdbIDPtr, mediaType)
				if err != nil {
					slog.Warn("Failed to find Overseerr request", "error", err, "tmdb_id", tmdbIDPtr, "tvdb_id", tvdbIDPtr)
				} else if req != nil {
					requestID = req.ID
					slog.Info("Found Overseerr request by media ID", "request_id", requestID, "tmdb_id", tmdbIDPtr, "tvdb_id", tvdbIDPtr)
				} else {
					slog.Info("No Overseerr request found for media", "tmdb_id", tmdbIDPtr, "tvdb_id", tvdbIDPtr)
				}
			}
		}

		// Delete the request if we found one
		if requestID > 0 {
			if err := s.overseerr.DeleteRequest(requestID); err != nil {
				errors = append(errors, fmt.Sprintf("failed to delete from Overseerr: %v", err))
				slog.Error("Failed to delete from Overseerr", "error", err, "request_id", requestID)
			} else {
				slog.Info("Deleted from Overseerr", "request_id", requestID)
			}
		} else {
			slog.Info("No Overseerr request ID available, skipping Overseerr deletion")
		}
	}

	// Step 5: Delete torrents from qBittorrent
	// Get all torrents associated with this media item
	var torrentHashes []string
	rows, err := s.db.QueryContext(ctx, `
		SELECT hash FROM torrents WHERE media_item_id = $1
	`, mediaID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var hash string
			if err := rows.Scan(&hash); err == nil {
				torrentHashes = append(torrentHashes, hash)
			}
		}
	}

	if s.qbittorrent != nil {
		for _, hash := range torrentHashes {
			if err := s.qbittorrent.DeleteTorrent(hash, true); err != nil {
				errors = append(errors, fmt.Sprintf("failed to delete torrent %s: %v", hash, err))
				slog.Error("Failed to delete torrent", "hash", hash, "error", err)
			} else {
				slog.Info("Deleted torrent", "hash", hash)
			}
		}
	}

	// Step 6: Log to audit log
	details := fmt.Sprintf("Deleted media: %s (type: %s)", title, mediaType)
	if len(errors) > 0 {
		details += fmt.Sprintf(" - Errors: %v", errors)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO audit_logs (user_id, action, media_item_id, media_title, media_type, details)
		VALUES ($1, 'delete', $2, $3, $4, $5)
	`, userID, mediaID, title, mediaType, details)
	if err != nil {
		slog.Error("Failed to create audit log", "error", err)
	}

	// Step 7: Delete from database
	_, err = s.db.ExecContext(ctx, `DELETE FROM media_items WHERE id = $1`, mediaID)
	if err != nil {
		return fmt.Errorf("failed to delete from database: %w", err)
	}

	slog.Info("Media deletion completed", "media_id", mediaID, "title", title, "errors", len(errors))
	
	if len(errors) > 0 {
		return fmt.Errorf("deletion completed with errors: %v", errors)
	}

	return nil
}

// deleteFiles deletes files from the filesystem
// This is a critical step - files MUST be deleted from disk as per requirements
func (s *DeletionService) deleteFiles(filePath string) error {
	// Check if path exists
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		slog.Warn("File path does not exist, skipping deletion", "path", filePath)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}

	if info.IsDir() {
		// Delete directory and all contents recursively
		slog.Info("Deleting directory and all contents", "path", filePath)
		if err := os.RemoveAll(filePath); err != nil {
			return fmt.Errorf("failed to delete directory: %w", err)
		}
		slog.Info("Successfully deleted directory", "path", filePath)
		return nil
	}

	// Delete single file
	slog.Info("Deleting file", "path", filePath)
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	slog.Info("Successfully deleted file", "path", filePath)

	// Try to remove parent directory if it's empty (e.g., for movie folders like /movies/Movie Name (Year)/)
	// This cleans up empty movie/series folders
	parentDir := filepath.Dir(filePath)
	if parentInfo, err := os.Stat(parentDir); err == nil && parentInfo.IsDir() {
		// Check if directory is empty
		entries, err := os.ReadDir(parentDir)
		if err == nil && len(entries) == 0 {
			slog.Info("Removing empty parent directory", "path", parentDir)
			if err := os.Remove(parentDir); err != nil {
				slog.Warn("Failed to remove empty parent directory", "path", parentDir, "error", err)
				// Don't fail the whole deletion if we can't remove empty dir
			}
		}
	}

	return nil
}

