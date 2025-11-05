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
		// This is a simple match - could be improved
		var mediaItemID sql.NullInt64
		
		// Try to match by content path
		if torrent.ContentPath != "" {
			err := s.db.QueryRowContext(ctx,
				`SELECT id FROM media_items 
				WHERE file_path LIKE $1 OR file_path = $2
				LIMIT 1`,
				torrent.ContentPath+"%",
				torrent.ContentPath,
			).Scan(&mediaItemID)
			
			if err != nil && err != sql.ErrNoRows {
				slog.Debug("Error matching torrent to media", "hash", torrent.Hash, "error", err)
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

			_, err = s.db.ExecContext(ctx,
				`UPDATE torrents SET
					tracker_id = $2,
					tracker_name = $3,
					tracker_type = $4,
					added_date = $5,
					seeding_time_seconds = $6,
					upload_bytes = $7,
					download_bytes = $8,
					ratio = $9,
					seeding_required_seconds = $10,
					seeding_required_ratio = $11,
					is_seeding = $12,
					last_synced_at = CURRENT_TIMESTAMP
				WHERE hash = $1`,
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
				slog.Error("Failed to update torrent", "error", err, "hash", torrent.Hash)
				continue
			}
		}
	}

	slog.Info("qBittorrent sync complete", "count", len(torrents))
	return nil
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

