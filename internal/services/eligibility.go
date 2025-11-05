package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"removarr/internal/integrations"
)

type EligibilityService struct {
	db          *sql.DB
	integrations *integrations.Client
}

type EligibilityStatus struct {
	IsEligible      bool
	Reason          string
	SeedingTime     int64  // in seconds
	RequiredTime    *int64 // in seconds, nil means infinite
	SeedingRatio    float64
	RequiredRatio   *float64
	TrackerType     string // "public" or "private"
	IsSeeding       bool
	LastWatched     *time.Time
	PlayCount       int
}

func NewEligibilityService(db *sql.DB, integrationsClient *integrations.Client) *EligibilityService {
	return &EligibilityService{
		db:          db,
		integrations: integrationsClient,
	}
}

// CheckEligibility determines if a media item is eligible for deletion
func (s *EligibilityService) CheckEligibility(ctx context.Context, mediaItemID int) (*EligibilityStatus, error) {
	status := &EligibilityStatus{
		IsEligible: false,
	}

	// Get media item
	var mediaItem struct {
		ID       int
		Type     string
		SonarrID *int
		RadarrID *int
		FilePath string
	}

	err := s.db.QueryRowContext(ctx,
		"SELECT id, type, sonarr_id, radarr_id, file_path FROM media_items WHERE id = $1",
		mediaItemID,
	).Scan(&mediaItem.ID, &mediaItem.Type, &mediaItem.SonarrID, &mediaItem.RadarrID, &mediaItem.FilePath)

	if err != nil {
		return nil, fmt.Errorf("media item not found: %w", err)
	}

	// Get all torrents for this media item
	rows, err := s.db.QueryContext(ctx,
		`SELECT hash, tracker_id, tracker_name, tracker_type, 
			seeding_time_seconds, ratio, seeding_required_seconds, 
			seeding_required_ratio, is_seeding
		FROM torrents WHERE media_item_id = $1`,
		mediaItemID,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query torrents: %w", err)
	}
	defer rows.Close()

	var torrents []struct {
		Hash                string
		TrackerID           *int
		TrackerName         *string
		TrackerType         *string
		SeedingTime         int64
		Ratio               float64
		RequiredTime        *int64
		RequiredRatio       *float64
		IsSeeding           bool
	}

	for rows.Next() {
		var t struct {
			Hash                string
			TrackerID           sql.NullInt64
			TrackerName         sql.NullString
			TrackerType         sql.NullString
			SeedingTime         int64
			Ratio               float64
			RequiredTime        sql.NullInt64
			RequiredRatio       sql.NullFloat64
			IsSeeding           bool
		}

		err := rows.Scan(&t.Hash, &t.TrackerID, &t.TrackerName, &t.TrackerType,
			&t.SeedingTime, &t.Ratio, &t.RequiredTime, &t.RequiredRatio, &t.IsSeeding)
		if err != nil {
			continue
		}

		torrent := struct {
			Hash                string
			TrackerID           *int
			TrackerName         *string
			TrackerType         *string
			SeedingTime         int64
			Ratio               float64
			RequiredTime        *int64
			RequiredRatio       *float64
			IsSeeding           bool
		}{
			Hash:        t.Hash,
			SeedingTime: t.SeedingTime,
			Ratio:       t.Ratio,
			IsSeeding:   t.IsSeeding,
		}

		if t.TrackerID.Valid {
			id := int(t.TrackerID.Int64)
			torrent.TrackerID = &id
		}
		if t.TrackerName.Valid {
			torrent.TrackerName = &t.TrackerName.String
		}
		if t.TrackerType.Valid {
			torrent.TrackerType = &t.TrackerType.String
		}
		if t.RequiredTime.Valid {
			rt := int64(t.RequiredTime.Int64)
			torrent.RequiredTime = &rt
		}
		if t.RequiredRatio.Valid {
			rr := t.RequiredRatio.Float64
			torrent.RequiredRatio = &rr
		}

		torrents = append(torrents, torrent)
	}

	if len(torrents) == 0 {
		status.Reason = "No torrents found for this media item"
		return status, nil
	}

	// Check each torrent's eligibility
	allEligible := true
	for _, torrent := range torrents {
		torrentEligible, reason := s.checkTorrentEligibility(torrent)
		if !torrentEligible {
			allEligible = false
			status.Reason = reason
			break
		}
	}

	status.IsEligible = allEligible
	if allEligible {
		status.Reason = "All seeding requirements met"
	}

	// Use the first torrent's stats for display
	if len(torrents) > 0 {
		t := torrents[0]
		status.SeedingTime = t.SeedingTime
		status.RequiredTime = t.RequiredTime
		status.SeedingRatio = t.Ratio
		status.RequiredRatio = t.RequiredRatio
		status.TrackerType = "unknown"
		if t.TrackerType != nil {
			status.TrackerType = *t.TrackerType
		}
		status.IsSeeding = t.IsSeeding
	}

	return status, nil
}

func (s *EligibilityService) checkTorrentEligibility(torrent struct {
	Hash                string
	TrackerID           *int
	TrackerName         *string
	TrackerType         *string
	SeedingTime         int64
	Ratio               float64
	RequiredTime        *int64
	RequiredRatio       *float64
	IsSeeding           bool
}) (bool, string) {
	// Public trackers: eligible by default (unless overridden)
	if torrent.TrackerType != nil && *torrent.TrackerType == "public" {
		// Check if there's an override
		if torrent.RequiredTime != nil || torrent.RequiredRatio != nil {
			// Has override, check requirements
			if torrent.RequiredTime != nil && torrent.SeedingTime < *torrent.RequiredTime {
				return false, fmt.Sprintf("Seeding time %ds < required %ds", torrent.SeedingTime, *torrent.RequiredTime)
			}
			if torrent.RequiredRatio != nil && torrent.Ratio < *torrent.RequiredRatio {
				return false, fmt.Sprintf("Ratio %.2f < required %.2f", torrent.Ratio, *torrent.RequiredRatio)
			}
		}
		// Public tracker with no override = eligible
		return true, "Public tracker - eligible"
	}

	// Private trackers: must meet requirements
	if torrent.RequiredTime != nil {
		if torrent.SeedingTime < *torrent.RequiredTime {
			return false, fmt.Sprintf("Seeding time %ds < required %ds", torrent.SeedingTime, *torrent.RequiredTime)
		}
	} else {
		// No time requirement specified = infinite (not eligible)
		return false, "Infinite seeding time required"
	}

	if torrent.RequiredRatio != nil {
		if torrent.Ratio < *torrent.RequiredRatio {
			return false, fmt.Sprintf("Ratio %.2f < required %.2f", torrent.Ratio, *torrent.RequiredRatio)
		}
	}

	// Must still be seeding
	if !torrent.IsSeeding {
		return false, "Torrent is not currently seeding"
	}

	return true, "All requirements met"
}

