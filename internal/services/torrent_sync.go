package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"removarr/internal/integrations"
)

type TorrentSyncService struct {
	db          *sql.DB
	integrations *integrations.Client
}

func NewTorrentSyncService(db *sql.DB, integrationsClient *integrations.Client) *TorrentSyncService {
	return &TorrentSyncService{
		db:          db,
		integrations: integrationsClient,
	}
}

// SyncFromQBittorrent fetches torrents from qBittorrent and updates the database
func (s *TorrentSyncService) SyncFromQBittorrent(ctx context.Context) error {
	if s.integrations.QBittorrent == nil {
		return fmt.Errorf("qbittorrent integration not enabled")
	}

	slog.Info("Syncing torrents from qBittorrent...")
	torrents, err := s.integrations.QBittorrent.GetTorrents()
	if err != nil {
		return fmt.Errorf("failed to fetch torrents from qBittorrent: %w", err)
	}

	// Get all indexers from Prowlarr to map tracker names to IDs
	indexerMap := make(map[string]*integrations.ProwlarrIndexer)
	if s.integrations.Prowlarr != nil {
		indexers, err := s.integrations.Prowlarr.GetIndexers()
		if err == nil {
			for i := range indexers {
				indexerMap[indexers[i].Name] = &indexers[i]
			}
		}
	}

	for _, torrent := range torrents {
		// Try to match torrent to media item by file path
		// Use multiple matching strategies for better reliability
		var mediaItemID sql.NullInt64
		
		if torrent.ContentPath != "" {
			// Strategy 1: Exact match
			err := s.db.QueryRowContext(ctx,
				`SELECT id FROM media_items 
				WHERE file_path = $1
				LIMIT 1`,
				torrent.ContentPath,
			).Scan(&mediaItemID)
			
			if err == nil && mediaItemID.Valid {
				// Found exact match
			} else if err == sql.ErrNoRows {
				// Strategy 2: Media item path is contained in torrent content path
				// (e.g., torrent: /data/downloads/Movie Title (2023), media: /data/downloads/Movie Title (2023)/Movie.Title.2023.mkv)
				err = s.db.QueryRowContext(ctx,
					`SELECT id FROM media_items 
					WHERE file_path LIKE $1 || '%' AND file_path != ''
					LIMIT 1`,
					torrent.ContentPath,
				).Scan(&mediaItemID)
				
				if err == nil && mediaItemID.Valid {
					// Found by containment
				} else if err == sql.ErrNoRows {
					// Strategy 3: Torrent content path is contained in media item path
					// (e.g., torrent: /data/downloads/Movie Title (2023), media: /data/downloads/Movie Title (2023)/Movie.Title.2023.mkv)
					err = s.db.QueryRowContext(ctx,
						`SELECT id FROM media_items 
						WHERE $1 LIKE file_path || '%' AND file_path != ''
						LIMIT 1`,
						torrent.ContentPath,
					).Scan(&mediaItemID)
					
					if err == nil && mediaItemID.Valid {
						// Found by reverse containment
					} else if err == sql.ErrNoRows {
						// Strategy 4: Match by directory name (basename of parent directory)
						// Extract the directory name from the torrent path
						// This is a fallback for when paths don't match exactly
						err = s.db.QueryRowContext(ctx,
							`SELECT id FROM media_items 
							WHERE file_path LIKE '%' || $1 || '%' AND file_path != ''
							ORDER BY 
								CASE WHEN file_path LIKE $1 || '%' THEN 1 ELSE 2 END,
								LENGTH(file_path) ASC
							LIMIT 1`,
							torrent.ContentPath,
						).Scan(&mediaItemID)
					}
				}
			}
			
			if err != nil && err != sql.ErrNoRows {
				slog.Debug("Error matching torrent to media", "hash", torrent.Hash, "error", err)
			}
		}
		
		// If still no match, try to match by torrent name (contains media title)
		// This is a last resort fallback
		if !mediaItemID.Valid && torrent.Name != "" {
			// Extract a potential title from torrent name (remove common suffixes)
			// This is heuristic-based and may have false positives
			err := s.db.QueryRowContext(ctx,
				`SELECT id FROM media_items 
				WHERE title = ANY(string_to_array($1, ' ')) 
				   OR $1 LIKE '%' || title || '%'
				ORDER BY 
					CASE WHEN title = ANY(string_to_array($1, ' ')) THEN 1 ELSE 2 END
				LIMIT 1`,
				torrent.Name,
			).Scan(&mediaItemID)
			
			if err != nil && err != sql.ErrNoRows {
				slog.Debug("Error matching torrent by name", "hash", torrent.Hash, "name", torrent.Name, "error", err)
			}
		}

		// Get tracker info from Prowlarr if available
		var trackerID *int
		var trackerName *string
		var trackerType *string
		var requiredTime *int64
		var requiredRatio *float64

		if torrent.Tracker != "" && s.integrations.Prowlarr != nil {
			// Try to find matching indexer
			for name, indexer := range indexerMap {
				if torrent.Tracker == name || torrent.Tracker == indexer.Name {
					trackerID = &indexer.ID
					trackerName = &indexer.Name
					trackerType = &indexer.Privacy
					
					if indexer.MinSeedTime != nil {
						rt := *indexer.MinSeedTime
						requiredTime = &rt
					}
					if indexer.MinRatio != nil {
						rr := *indexer.MinRatio
						requiredRatio = &rr
					}
					break
				}
			}
			
			// If not found, try to determine if it's public/private from URL
			if trackerType == nil {
				isPublic := s.isPublicTracker(torrent.Tracker)
				trackerTypeStr := "private"
				if isPublic {
					trackerTypeStr = "public"
				}
				trackerType = &trackerTypeStr
				trackerName = &torrent.Tracker
			}
		}

		// Check if torrent exists
		var existingHash string
		err = s.db.QueryRowContext(ctx,
			"SELECT hash FROM torrents WHERE hash = $1",
			torrent.Hash,
		).Scan(&existingHash)

		addedDate := time.Unix(torrent.AddedOn, 0)
		isSeeding := torrent.State == "uploading" || torrent.State == "stalledUP"

		if err == sql.ErrNoRows {
			// Insert new torrent
			var mediaID interface{}
			if mediaItemID.Valid {
				mediaID = mediaItemID.Int64
			}
			
			var trackerIDVal interface{}
			if trackerID != nil {
				trackerIDVal = *trackerID
			}

			_, err = s.db.ExecContext(ctx,
				`INSERT INTO torrents 
					(media_item_id, hash, tracker_id, tracker_name, tracker_type,
					added_date, seeding_time_seconds, upload_bytes, download_bytes,
					ratio, seeding_required_seconds, seeding_required_ratio, is_seeding,
					last_synced_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, CURRENT_TIMESTAMP)`,
				mediaID,
				torrent.Hash,
				trackerIDVal,
				trackerName,
				trackerType,
				addedDate,
				torrent.SeedingTime,
				torrent.Uploaded,
				torrent.Downloaded,
				torrent.Ratio,
				requiredTime,
				requiredRatio,
				isSeeding,
			)
			if err != nil {
				slog.Error("Failed to insert torrent", "error", err, "hash", torrent.Hash)
				continue
			}
		} else if err == nil {
			// Update existing torrent
			var trackerIDVal interface{}
			if trackerID != nil {
				trackerIDVal = *trackerID
			}
			
			var mediaIDVal interface{}
			// If torrent exists but wasn't linked before, try to link it now
			if mediaItemID.Valid {
				// Check if torrent already has a different media_item_id
				var currentMediaID sql.NullInt64
				err := s.db.QueryRowContext(ctx,
					"SELECT media_item_id FROM torrents WHERE hash = $1",
					torrent.Hash,
				).Scan(&currentMediaID)
				
				if err == nil {
					// If no media_item_id set, or if it's different and the new one is valid, update it
					if !currentMediaID.Valid || (mediaItemID.Valid && currentMediaID.Int64 != mediaItemID.Int64) {
						mediaIDVal = mediaItemID.Int64
					} else {
						mediaIDVal = currentMediaID.Int64 // Keep existing link
					}
				}
			}

			_, err = s.db.ExecContext(ctx,
				`UPDATE torrents SET
					media_item_id = COALESCE($2, media_item_id),
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
					last_synced_at = CURRENT_TIMESTAMP
				WHERE hash = $1`,
				torrent.Hash,
				mediaIDVal,
				trackerIDVal,
				trackerName,
				trackerType,
				addedDate,
				torrent.SeedingTime,
				torrent.Uploaded,
				torrent.Downloaded,
				torrent.Ratio,
				requiredTime,
				requiredRatio,
				isSeeding,
			)
			if err != nil {
				slog.Error("Failed to update torrent", "error", err, "hash", torrent.Hash)
				continue
			}
			
			if mediaItemID.Valid && mediaIDVal != nil {
				slog.Debug("Linked existing torrent to media item", 
					"hash", torrent.Hash, 
					"media_item_id", mediaItemID.Int64,
					"content_path", torrent.ContentPath)
			}
		}
	}

	slog.Info("qBittorrent sync complete", "count", len(torrents))
	
	// After syncing, try to link any unlinked torrents to media items
	// This helps catch cases where file paths didn't match initially
	s.logUnlinkedTorrents(ctx)
	
	return nil
}

// logUnlinkedTorrents logs statistics about unlinked torrents for debugging
func (s *TorrentSyncService) logUnlinkedTorrents(ctx context.Context) {
	var unlinkedCount int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM torrents WHERE media_item_id IS NULL",
	).Scan(&unlinkedCount)
	if err == nil && unlinkedCount > 0 {
		slog.Warn("Unlinked torrents detected", "count", unlinkedCount,
			"hint", "Torrents may not match media item file paths. Check file path configurations.")
	}
}

// isPublicTracker checks if a tracker URL is likely a public tracker
func (s *TorrentSyncService) isPublicTracker(trackerURL string) bool {
	publicTrackers := []string{
		"1337x",
		"rarbg",
		"thepiratebay",
		"torrentz",
		"kickass",
		"yts",
		"eztv",
		"nyaa",
	}

	trackerLower := fmt.Sprintf("%v", trackerURL)
	for _, public := range publicTrackers {
		if len(trackerLower) > len(public) && trackerLower[:len(public)] == public {
			return true
		}
	}

	return false
}

