package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"removarr/internal/integrations"
	"removarr/internal/services"

	"golang.org/x/crypto/bcrypt"
	"github.com/gorilla/mux"
)

// Placeholder handlers - will be implemented fully

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	// Check if setup is needed (no users exist)
	var userCount int
	err := s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&userCount)
	if err != nil {
		slog.Error("Failed to check setup status", "error", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// If users exist, redirect to dashboard
	if userCount > 0 {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	// Handle POST - create first admin user
	if r.Method == "POST" {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Email    string `json:"email"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if req.Username == "" || req.Password == "" {
			http.Error(w, "Username and password are required", http.StatusBadRequest)
			return
		}

		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Failed to hash password", http.StatusInternalServerError)
			return
		}

		// Create admin user
		var email sql.NullString
		if req.Email != "" {
			email = sql.NullString{String: req.Email, Valid: true}
		}

		_, err = s.db.ExecContext(r.Context(),
			"INSERT INTO users (username, email, password_hash, is_admin, is_active) VALUES ($1, $2, $3, $4, $5)",
			req.Username, email, string(hashedPassword), true, true,
		)
		if err != nil {
			slog.Error("Failed to create admin user", "error", err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		slog.Info("First admin user created", "username", req.Username)

		// Return success - frontend will redirect to login
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Admin user created successfully",
		})
		return
	}

	// GET - show setup wizard
	// Note: Integration settings are now in database, not config file
	// Setup wizard just needs to create first admin user
	data := map[string]interface{}{
		"User": nil,
	}
	if err := s.renderTemplate(w, "setup.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		slog.Error("Template render error", "error", err)
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Check if setup is needed first
	var userCount int
	err := s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&userCount)
	if err == nil && userCount == 0 {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}

	// Check if user is authenticated
	session, err := s.store.Get(r, sessionKey)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	
	userID, ok := session.Values[userIDKey].(int)
	if !ok || userID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to dashboard
	session, err := s.store.Get(r, sessionKey)
	if err == nil {
		if userID, ok := session.Values[userIDKey].(int); ok && userID > 0 {
			slog.Info("Already logged in, redirecting to dashboard", "user_id", userID)
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
	}

	// Render login page
	// Since login.html is parsed last, its "content" definition will be used by base.html
	data := map[string]interface{}{
		"User": nil, // No user for login page
	}
	slog.Info("Rendering login page")
	if err := s.renderTemplate(w, "login.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		slog.Error("Template render error", "error", err)
	}
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Always sync on dashboard load (background, non-blocking)
	// Only on full page loads, not HTMX requests
	if r.Header.Get("HX-Request") == "" {
		go func() {
			ctx := context.Background()
			slog.Info("Triggering background sync on dashboard load")
			if err := s.mediaSync.SyncAll(ctx); err != nil {
				slog.Error("Background auto-sync failed", "error", err)
			} else {
				slog.Info("Background auto-sync completed successfully")
			}
			// Also sync torrents
			if err := s.torrentSync.SyncFromQBittorrent(ctx); err != nil {
				slog.Error("Background torrent sync failed", "error", err)
			}
		}()
	}
	
	// Get last sync time for display
	var lastSyncTime sql.NullTime
	s.db.QueryRowContext(r.Context(),
		"SELECT MAX(last_synced_at) FROM media_items",
	).Scan(&lastSyncTime)
	
	// Get filters from query params
	mediaType := r.URL.Query().Get("type")
	eligible := r.URL.Query().Get("eligible")
	downloaded := r.URL.Query().Get("downloaded")
	
	// Pagination
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	pageSize := 50 // Items per page
	offset := (page - 1) * pageSize

	// Build query - first get total count
	countQuery := "SELECT COUNT(*) FROM media_items WHERE 1=1"
	countArgs := []interface{}{}
	countArgPos := 1

	if mediaType != "" {
		countQuery += fmt.Sprintf(" AND type = $%d", countArgPos)
		countArgs = append(countArgs, mediaType)
		countArgPos++
	}

	var totalCount int
	err := s.db.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		slog.Error("Failed to get media count", "error", err)
		totalCount = 0
	}

	// Build main query
	query := "SELECT id, title, type, tmdb_id, tvdb_id, sonarr_id, radarr_id, overseerr_request_id, requested_by_user_id, file_path, file_size, added_date, last_synced_at FROM media_items WHERE 1=1"
	args := []interface{}{}
	argPos := 1

	if mediaType != "" {
		query += fmt.Sprintf(" AND type = $%d", argPos)
		args = append(args, mediaType)
		argPos++
	}

	query += fmt.Sprintf(" ORDER BY added_date DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, pageSize, offset)

	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type MediaItem struct {
		ID              int
		Title           string
		Type            string
		FileSize        int64
		FilePath        string
		SeedingTime     int64
		SeedingRatio    float64
		TrackerType     string
		Eligible        bool
		EligibilityReason string
		Downloaded      bool
		RadarrID        *int
		SonarrID        *int
		OverseerrRequestID *int
		TMDBID          *int
		RadarrURL       string
		SonarrURL       string
		OverseerrURL    string
		PosterURL       string
	}

	mediaItems := []MediaItem{} // Initialize as empty slice, not nil
	
	// Debug: Log what we're querying
	slog.Info("Dashboard query", "sql", query, "args", args, "mediaType", mediaType, "eligible", eligible, "downloaded", downloaded)
	
	for rows.Next() {
		var item struct {
			ID                 int
			Title              string
			Type               string
			TMDBID             sql.NullInt64
			TVDBID             sql.NullInt64
			SonarrID           sql.NullInt64
			RadarrID           sql.NullInt64
			OverseerrRequestID sql.NullInt64
			RequestedByUserID  sql.NullInt64
			FilePath           sql.NullString
			FileSize           sql.NullInt64
			AddedDate          sql.NullTime
			LastSyncedAt       time.Time
		}

		err := rows.Scan(&item.ID, &item.Title, &item.Type, &item.TMDBID, &item.TVDBID,
			&item.SonarrID, &item.RadarrID, &item.OverseerrRequestID, &item.RequestedByUserID,
			&item.FilePath, &item.FileSize, &item.AddedDate, &item.LastSyncedAt)
		if err != nil {
			slog.Error("Error scanning media row", "error", err)
			continue
		}
		
		slog.Info("Processing media item from DB", "id", item.ID, "title", item.Title, "type", item.Type, "file_size", item.FileSize.Int64)

		// Check eligibility - this should never error for "no torrents" case
		// (it returns status with reason, not an error)
		eligibility, err := s.eligibility.CheckEligibility(r.Context(), item.ID)
		if err != nil {
			// Only real errors (like DB issues) should reach here
			slog.Debug("Eligibility check error", "media_id", item.ID, "error", err)
			eligibility = &services.EligibilityStatus{
				IsEligible: false,
				Reason: fmt.Sprintf("Error: %v", err),
				SeedingTime: 0,
				SeedingRatio: 0.0,
				TrackerType: "",
			}
		}

		// Apply filters
		filteredOut := false
		if eligible == "true" && !eligibility.IsEligible {
			slog.Info("Filtering out - not eligible", "title", item.Title)
			filteredOut = true
		}
		if eligible == "false" && eligibility.IsEligible {
			slog.Info("Filtering out - eligible when filtered for not eligible", "title", item.Title)
			filteredOut = true
		}
		if downloaded == "true" && item.FileSize.Int64 == 0 {
			slog.Info("Filtering out - not downloaded when filtered for downloaded", "title", item.Title)
			filteredOut = true
		}
		if downloaded == "false" && item.FileSize.Int64 > 0 {
			slog.Info("Filtering out - downloaded when filtered for not downloaded", "title", item.Title)
			filteredOut = true
		}
		
		if filteredOut {
			continue
		}

		isDownloaded := item.FileSize.Int64 > 0 && item.FilePath.Valid && item.FilePath.String != ""

		// Build URLs for Radarr, Sonarr, and Overseerr
		var radarrURL, sonarrURL, overseerrURL, posterURL string
		var radarrID *int
		var sonarrID *int
		var overseerrRequestID *int
		var tmdbID *int

		if item.RadarrID.Valid {
			id := int(item.RadarrID.Int64)
			radarrID = &id
			// Radarr uses TMDB ID in the URL, not Radarr ID
			if item.TMDBID.Valid {
				radarrURL = fmt.Sprintf("%s/movie/%d", s.config.Radarr.URL, item.TMDBID.Int64)
			}
		}
		if item.SonarrID.Valid {
			id := int(item.SonarrID.Int64)
			sonarrID = &id
			// Sonarr uses TVDB ID in the URL, not Sonarr ID
			if item.TVDBID.Valid {
				sonarrURL = fmt.Sprintf("%s/series/%d", s.config.Sonarr.URL, item.TVDBID.Int64)
			}
		}
		if item.OverseerrRequestID.Valid {
			id := int(item.OverseerrRequestID.Int64)
			overseerrRequestID = &id
			overseerrURL = fmt.Sprintf("%s/requests/%d", s.config.Overseerr.URL, id)
		}
		
		// Generate poster URL
		// Use TMDB API for posters (works from anywhere, not just internal services)
		if item.TMDBID.Valid {
			// TMDB poster API: https://image.tmdb.org/t/p/w500/{poster_path}
			// We'll need to fetch poster_path from Radarr/Sonarr API or use a placeholder
			// For now, use TMDB directly - but we need the poster_path
			// Fallback: Use Radarr/Sonarr if available, but proxy through our server
			if item.RadarrID.Valid && s.config.Radarr.Enabled && s.config.Radarr.URL != "" {
				// Proxy through our server so it works from browser
				posterURL = fmt.Sprintf("/api/poster/radarr/%d", item.RadarrID.Int64)
			} else if item.SonarrID.Valid && s.config.Sonarr.Enabled && s.config.Sonarr.URL != "" {
				// Proxy through our server so it works from browser
				posterURL = fmt.Sprintf("/api/poster/sonarr/%d", item.SonarrID.Int64)
			} else if item.TMDBID.Valid {
				// Fallback: Use TMDB API (no API key needed for images)
				// We'll need to fetch the actual poster path, but for now use placeholder
				// TODO: Fetch actual poster path from TMDB API
			}
		} else {
			// Fallback to Radarr/Sonarr direct URLs if TMDB not available
			if item.RadarrID.Valid && s.config.Radarr.Enabled && s.config.Radarr.URL != "" {
				posterURL = fmt.Sprintf("/api/poster/radarr/%d", item.RadarrID.Int64)
			} else if item.SonarrID.Valid && s.config.Sonarr.Enabled && s.config.Sonarr.URL != "" {
				posterURL = fmt.Sprintf("/api/poster/sonarr/%d", item.SonarrID.Int64)
			}
		}
		
		if item.TMDBID.Valid {
			id := int(item.TMDBID.Int64)
			tmdbID = &id
		}

		slog.Info("Adding media item to results", "title", item.Title, "type", item.Type, "downloaded", isDownloaded)
		mediaItems = append(mediaItems, MediaItem{
			ID:               item.ID,
			Title:            item.Title,
			Type:             item.Type,
			FileSize:         item.FileSize.Int64,
			FilePath:         item.FilePath.String,
			SeedingTime:      eligibility.SeedingTime,
			SeedingRatio:     eligibility.SeedingRatio,
			TrackerType:      eligibility.TrackerType,
			Eligible:         eligibility.IsEligible,
			EligibilityReason: eligibility.Reason,
			Downloaded:       isDownloaded,
			RadarrID:         radarrID,
			SonarrID:         sonarrID,
			OverseerrRequestID: overseerrRequestID,
			TMDBID:           tmdbID,
			RadarrURL:        radarrURL,
			SonarrURL:        sonarrURL,
			OverseerrURL:     overseerrURL,
			PosterURL:        posterURL,
		})
	}
	
	slog.Info("Processed media items", "count", len(mediaItems), "query_params", map[string]string{
		"type": mediaType,
		"eligible": eligible,
		"downloaded": downloaded,
	})

	// Calculate pagination info
	totalPages := (totalCount + pageSize - 1) / pageSize // Ceiling division
	if totalPages == 0 {
		totalPages = 1
	}

	// Check if this is an HTMX request (for partial updates)
	if r.Header.Get("HX-Request") != "" {
		// Return just the media list - render the template directly, not wrapped in base
		data := map[string]interface{}{
			"Media":      mediaItems,
			"Page":       page,
			"TotalPages": totalPages,
			"TotalCount": totalCount,
			"PageSize":   pageSize,
		}
		if err := templates.ExecuteTemplate(w, "media_list", data); err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
			slog.Error("Template render error", "error", err)
		}
		return
	}

	// Full page render - pass media items to dashboard template
	// Always pass Media as a slice, even if empty, so template can check length
	// Also pass User info for the nav bar
	authCtx, _ := r.Context().Value("auth").(AuthContext)
	
	firstItem := "none"
	if len(mediaItems) > 0 {
		firstItem = mediaItems[0].Title
	}
	
	// Format last sync time for display
	var lastSyncDisplay string
	if lastSyncTime.Valid {
		lastSyncDisplay = lastSyncTime.Time.Format("2006-01-02 15:04:05 MST")
	} else {
		lastSyncDisplay = "Never"
	}
	
	data := map[string]interface{}{
		"Media":        mediaItems,
		"Type":         mediaType, // Pass current filter values to template
		"Eligible":     eligible,
		"Downloaded":   downloaded,
		"Page":         page,
		"TotalPages":   totalPages,
		"TotalCount":   totalCount,
		"PageSize":     pageSize,
		"User": map[string]interface{}{
			"Username": authCtx.Username,
			"IsAdmin": authCtx.IsAdmin,
		},
		"LastSyncTime": lastSyncDisplay,
	}
	
	slog.Info("Rendering dashboard", "media_count", len(mediaItems), "first_item", firstItem, "filters", map[string]string{
		"type": mediaType,
		"eligible": eligible,
		"downloaded": downloaded,
	})
	
	// For dashboard, we need to ensure dashboard.html's content is used
	// Since dashboard.html is parsed before login.html, we need to re-parse it
	// OR use a different template structure. For now, let's try parsing dashboard.html
	// again before rendering, or use a wrapper approach.
	
	// Actually, let's swap the parse order: parse login.html first, then dashboard.html
	// This way dashboard.html's definitions win (which is what we want for dashboard)
	// But then login page won't work...
	
	// Better solution: Use unique template names or restructure templates
	// For now, let's ensure we parse dashboard.html AFTER login.html when rendering dashboard
	// But we can't re-parse at runtime easily...
	
	// Temporary fix: Parse dashboard.html last so its content wins
	// But we need to re-order the template parsing
	if err := s.renderTemplate(w, "dashboard.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		slog.Error("Template render error", "error", err)
	}
}

func (s *Server) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	// Get auth context
	authCtx, ok := r.Context().Value("auth").(AuthContext)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := map[string]interface{}{
		"User": authCtx,
	}

	if err := s.renderTemplate(w, "admin.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		slog.Error("Template render error", "error", err)
	}
}

func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	// Get auth context
	authCtx, ok := r.Context().Value("auth").(AuthContext)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get sync frequency from database
	var syncFrequency string
	err := s.db.QueryRowContext(r.Context(),
		"SELECT value FROM settings WHERE key = 'sync_frequency'",
	).Scan(&syncFrequency)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("Failed to get sync frequency", "error", err)
	}
	if syncFrequency == "" {
		syncFrequency = "5m" // Default
	}

	// Get qBittorrent stats
	var qbitStats map[string]interface{}
	if s.config.QBittorrent.Enabled {
		qbitStats = s.getQBittorrentStats(r.Context())
	} else {
		qbitStats = map[string]interface{}{
			"TotalTorrents":   0,
			"SeedingTorrents": 0,
			"TotalUpload":     0,
			"TotalDownload":   0,
			"LastSync":        nil,
		}
	}

	data := map[string]interface{}{
		"User": authCtx,
		"Config": s.config,
		"Settings": map[string]interface{}{
			"SyncFrequency": syncFrequency,
			"QBittorrentStats": qbitStats,
		},
	}

	if err := s.renderTemplate(w, "settings.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		slog.Error("Template render error", "error", err)
	}
}

// @Summary      List media items
// @Description  Get a list of all media items with optional filters
// @Tags         media
// @Accept       json
// @Produce      json
// @Param        user_id   query     int     false  "Filter by user ID"
// @Param        type      query     string  false  "Filter by type (movie/series)"
// @Param        sync      query     bool    false  "Sync from Sonarr/Radarr before listing"
// @Security     BasicAuth
// @Success      200       {array}   map[string]interface{}
// @Failure      401       {object}  map[string]string  "Unauthorized"
// @Router       /media [get]
func (s *Server) handleListMedia(w http.ResponseWriter, r *http.Request) {
	// Check if sync is requested
	if r.URL.Query().Get("sync") == "true" {
		ctx := r.Context()
		if err := s.mediaSync.SyncAll(ctx); err != nil {
			slog.Error("Media sync failed", "error", err)
			http.Error(w, "Media sync failed", http.StatusInternalServerError)
			return
		}
		if err := s.torrentSync.SyncFromQBittorrent(ctx); err != nil {
			slog.Error("Torrent sync failed", "error", err)
			// Don't fail the request, just log the error
		}
	}

	// Get filters
	userID := r.URL.Query().Get("user_id")
	mediaType := r.URL.Query().Get("type")

	// Build query
	query := "SELECT id, title, type, tmdb_id, tvdb_id, sonarr_id, radarr_id, overseerr_request_id, requested_by_user_id, file_path, file_size, added_date, last_synced_at FROM media_items WHERE 1=1"
	args := []interface{}{}
	argPos := 1

	if userID != "" {
		query += fmt.Sprintf(" AND requested_by_user_id = $%d", argPos)
		args = append(args, userID)
		argPos++
	}

	if mediaType != "" {
		query += fmt.Sprintf(" AND type = $%d", argPos)
		args = append(args, mediaType)
		argPos++
	}

	query += " ORDER BY added_date DESC LIMIT 100"

	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var item struct {
			ID                 int
			Title              string
			Type               string
			TMDBID             sql.NullInt64
			TVDBID             sql.NullInt64
			SonarrID           sql.NullInt64
			RadarrID           sql.NullInt64
			OverseerrRequestID sql.NullInt64
			RequestedByUserID  sql.NullInt64
			FilePath           sql.NullString
			FileSize           sql.NullInt64
			AddedDate          sql.NullTime
			LastSyncedAt       time.Time
		}

		err := rows.Scan(&item.ID, &item.Title, &item.Type, &item.TMDBID, &item.TVDBID,
			&item.SonarrID, &item.RadarrID, &item.OverseerrRequestID, &item.RequestedByUserID,
			&item.FilePath, &item.FileSize, &item.AddedDate, &item.LastSyncedAt)
		if err != nil {
			continue
		}

		// Determine if media is downloaded (has files)
		isDownloaded := item.FileSize.Int64 > 0 && item.FilePath.Valid && item.FilePath.String != ""

		result := map[string]interface{}{
			"id":            item.ID,
			"title":         item.Title,
			"type":          item.Type,
			"file_size":     item.FileSize.Int64,
			"added_date":    item.AddedDate.Time,
			"last_synced":   item.LastSyncedAt,
			"downloaded":    isDownloaded,
		}

		if item.TMDBID.Valid {
			result["tmdb_id"] = item.TMDBID.Int64
		}
		if item.TVDBID.Valid {
			result["tvdb_id"] = item.TVDBID.Int64
		}
		if item.SonarrID.Valid {
			result["sonarr_id"] = item.SonarrID.Int64
		}
		if item.RadarrID.Valid {
			result["radarr_id"] = item.RadarrID.Int64
		}
		if item.FilePath.Valid {
			result["file_path"] = item.FilePath.String
		}

		// Check eligibility
		eligibility, err := s.eligibility.CheckEligibility(r.Context(), item.ID)
		if err == nil {
			result["eligible"] = eligibility.IsEligible
			result["eligibility_reason"] = eligibility.Reason
			result["seeding_time"] = eligibility.SeedingTime
			result["seeding_ratio"] = eligibility.SeedingRatio
			result["tracker_type"] = eligibility.TrackerType
		}

		results = append(results, result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleDeleteMedia(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid media ID", http.StatusBadRequest)
		return
	}

	// TODO: Implement deletion workflow
	// 1. Get media item from DB
	// 2. Check eligibility
	// 3. Delete from filesystem
	// 4. Unmonitor from Sonarr/Radarr
	// 5. Delete from Overseerr
	// 6. Log to audit log

	_ = id
	http.Error(w, "Media deletion not yet implemented", http.StatusNotImplemented)
}

func (s *Server) handleBulkDeleteMedia(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int `json:"ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if len(req.IDs) == 0 {
		http.Error(w, "No media IDs provided", http.StatusBadRequest)
		return
	}

	// Get user ID from auth context
	authCtx, ok := r.Context().Value("auth").(AuthContext)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	errors := []string{}
	successCount := 0

	// Delete each media item
	for _, id := range req.IDs {
		if err := s.deletion.DeleteMediaItem(ctx, id, authCtx.UserID); err != nil {
			slog.Error("Failed to delete media item in bulk", "id", id, "error", err)
			errors = append(errors, fmt.Sprintf("Media ID %d: %v", id, err))
		} else {
			successCount++
		}
	}

	response := map[string]interface{}{
		"success": len(errors) == 0,
		"deleted": successCount,
		"total":   len(req.IDs),
	}
	if len(errors) > 0 {
		response["errors"] = errors
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// @Summary      List users
// @Description  Get a list of all users
// @Tags         admin
// @Produce      json
// @Security     BasicAuth
// @Success      200  {array}   map[string]interface{}
// @Failure      401  {object}  map[string]string  "Unauthorized"
// @Failure      403  {object}  map[string]string  "Forbidden"
// @Router       /admin/users [get]
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), "SELECT id, username, email, is_admin, is_active, created_at FROM users ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var user struct {
			ID        int
			Username  string
			Email     sql.NullString
			IsAdmin   bool
			IsActive  bool
			CreatedAt string
		}

		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.IsActive, &user.CreatedAt); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		userMap := map[string]interface{}{
			"id":        user.ID,
			"username":  user.Username,
			"is_admin":  user.IsAdmin,
			"is_active": user.IsActive,
			"created_at": user.CreatedAt,
		}

		if user.Email.Valid {
			userMap["email"] = user.Email.String
		}

		users = append(users, userMap)
	}

	json.NewEncoder(w).Encode(users)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
		IsAdmin  bool   `json:"is_admin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	var email sql.NullString
	if req.Email != "" {
		email = sql.NullString{String: req.Email, Valid: true}
	}

	_, err = s.db.ExecContext(r.Context(),
		"INSERT INTO users (username, email, password_hash, is_admin, is_active) VALUES ($1, $2, $3, $4, $5)",
		req.Username, email, string(hashedPassword), req.IsAdmin, true,
	)
	if err != nil {
		slog.Error("Failed to create user", "error", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "User created successfully",
	})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"` // Optional - only update if provided
		IsAdmin  *bool  `json:"is_admin"`
		IsActive *bool  `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Build update query dynamically
	updates := []string{}
	args := []interface{}{}
	argPos := 1

	if req.Username != "" {
		updates = append(updates, fmt.Sprintf("username = $%d", argPos))
		args = append(args, req.Username)
		argPos++
	}

	var email sql.NullString
	if req.Email != "" {
		email = sql.NullString{String: req.Email, Valid: true}
		updates = append(updates, fmt.Sprintf("email = $%d", argPos))
		args = append(args, email)
		argPos++
	}

	if req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Failed to hash password", http.StatusInternalServerError)
			return
		}
		updates = append(updates, fmt.Sprintf("password_hash = $%d", argPos))
		args = append(args, string(hashedPassword))
		argPos++
	}

	if req.IsAdmin != nil {
		updates = append(updates, fmt.Sprintf("is_admin = $%d", argPos))
		args = append(args, *req.IsAdmin)
		argPos++
	}

	if req.IsActive != nil {
		updates = append(updates, fmt.Sprintf("is_active = $%d", argPos))
		args = append(args, *req.IsActive)
		argPos++
	}

	if len(updates) == 0 {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, id)

	// Build SET clause properly
	setClause := ""
	for i, update := range updates {
		if i > 0 {
			setClause += ", "
		}
		setClause += update
	}
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d", setClause, argPos)

	_, err = s.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		slog.Error("Failed to update user", "error", err)
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "User updated successfully",
	})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Prevent deleting yourself
	authCtx, ok := r.Context().Value("auth").(AuthContext)
	if ok && authCtx.UserID == id {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
		return
	}

	_, err = s.db.ExecContext(r.Context(), "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		slog.Error("Failed to delete user", "error", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "User deleted successfully",
	})
}

func (s *Server) handleImportPlexUsers(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement Plex user import
	// This requires Plex integration to fetch users
	http.Error(w, "Plex user import not yet implemented", http.StatusNotImplemented)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	// Get settings from database (with config defaults as fallback)
	settings := map[string]interface{}{
		"overseerr": map[string]interface{}{
			"enabled": s.getSetting("overseerr.enabled", fmt.Sprintf("%t", s.config.Overseerr.Enabled)) == "true",
			"url":     s.getSetting("overseerr.url", s.config.Overseerr.URL),
			"api_key": s.getSetting("overseerr.api_key", s.config.Overseerr.APIKey),
		},
		"sonarr": map[string]interface{}{
			"enabled": s.getSetting("sonarr.enabled", fmt.Sprintf("%t", s.config.Sonarr.Enabled)) == "true",
			"url":     s.getSetting("sonarr.url", s.config.Sonarr.URL),
			"api_key": s.getSetting("sonarr.api_key", s.config.Sonarr.APIKey),
		},
		"radarr": map[string]interface{}{
			"enabled": s.getSetting("radarr.enabled", fmt.Sprintf("%t", s.config.Radarr.Enabled)) == "true",
			"url":     s.getSetting("radarr.url", s.config.Radarr.URL),
			"api_key": s.getSetting("radarr.api_key", s.config.Radarr.APIKey),
		},
		"prowlarr": map[string]interface{}{
			"enabled": s.getSetting("prowlarr.enabled", fmt.Sprintf("%t", s.config.Prowlarr.Enabled)) == "true",
			"url":     s.getSetting("prowlarr.url", s.config.Prowlarr.URL),
			"api_key": s.getSetting("prowlarr.api_key", s.config.Prowlarr.APIKey),
		},
		"qbittorrent": map[string]interface{}{
			"enabled":  s.getSetting("qbittorrent.enabled", fmt.Sprintf("%t", s.config.QBittorrent.Enabled)) == "true",
			"url":      s.getSetting("qbittorrent.url", s.config.QBittorrent.URL),
			"username": s.getSetting("qbittorrent.username", s.config.QBittorrent.Username),
			"password": "", // Never return password
		},
		"tautulli": map[string]interface{}{
			"enabled": s.getSetting("tautulli.enabled", fmt.Sprintf("%t", s.config.Tautulli.Enabled)) == "true",
			"url":     s.getSetting("tautulli.url", s.config.Tautulli.URL),
			"api_key": s.getSetting("tautulli.api_key", s.config.Tautulli.APIKey),
		},
		"sync_frequency": s.getSetting("sync_frequency", "5m"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	settingsUpdated := false

	// Handle sync_frequency setting
	if syncFreq, ok := req["sync_frequency"].(string); ok {
		// Validate duration format
		if _, err := time.ParseDuration(syncFreq); err != nil {
			http.Error(w, "Invalid sync frequency format (use format like '5m', '1h', '30s')", http.StatusBadRequest)
			return
		}
		
		if err := s.setSetting("sync_frequency", syncFreq, "string"); err != nil {
			slog.Error("Failed to save sync frequency", "error", err)
			http.Error(w, "Failed to save settings", http.StatusInternalServerError)
			return
		}
		settingsUpdated = true
		slog.Info("Sync frequency updated", "frequency", syncFreq)
	}

	// Handle integration settings - save to database
	integrationNames := []string{"overseerr", "sonarr", "radarr", "prowlarr", "qbittorrent", "tautulli"}
	for _, serviceName := range integrationNames {
		if serviceData, ok := req[serviceName].(map[string]interface{}); ok {
			enabled, _ := serviceData["enabled"].(bool)
			url, _ := serviceData["url"].(string)
			apiKey, _ := serviceData["api_key"].(string)
			username, _ := serviceData["username"].(string)
			password, _ := serviceData["password"].(string)
			
			// Save enabled state
			if err := s.setSetting(fmt.Sprintf("%s.enabled", serviceName), fmt.Sprintf("%t", enabled), "boolean"); err != nil {
				slog.Error("Failed to save setting", "key", fmt.Sprintf("%s.enabled", serviceName), "error", err)
				http.Error(w, "Failed to save settings", http.StatusInternalServerError)
				return
			}
			
			// Save URL if provided - strip trailing slash
			if url != "" {
				url = strings.TrimSuffix(url, "/")
				if err := s.setSetting(fmt.Sprintf("%s.url", serviceName), url, "string"); err != nil {
					slog.Error("Failed to save setting", "key", fmt.Sprintf("%s.url", serviceName), "error", err)
					http.Error(w, "Failed to save settings", http.StatusInternalServerError)
					return
				}
			}
			
			// Save API key if provided (only for services that use API keys)
			if apiKey != "" && serviceName != "qbittorrent" {
				if err := s.setSetting(fmt.Sprintf("%s.api_key", serviceName), apiKey, "string"); err != nil {
					slog.Error("Failed to save setting", "key", fmt.Sprintf("%s.api_key", serviceName), "error", err)
					http.Error(w, "Failed to save settings", http.StatusInternalServerError)
					return
				}
			}
			
			// Save username/password for qBittorrent
			if serviceName == "qbittorrent" {
				if username != "" {
					if err := s.setSetting("qbittorrent.username", username, "string"); err != nil {
						slog.Error("Failed to save setting", "key", "qbittorrent.username", "error", err)
						http.Error(w, "Failed to save settings", http.StatusInternalServerError)
						return
					}
				}
				if password != "" {
					if err := s.setSetting("qbittorrent.password", password, "string"); err != nil {
						slog.Error("Failed to save setting", "key", "qbittorrent.password", "error", err)
						http.Error(w, "Failed to save settings", http.StatusInternalServerError)
						return
					}
				}
			}
			
			settingsUpdated = true
		}
	}

	// Reload settings from database and update integrations
	if settingsUpdated {
		s.loadIntegrationSettings()
		s.integrations = integrations.NewClient(s.config)
		// Update services that depend on integrations
		s.mediaSync = services.NewMediaSyncService(s.db, s.integrations)
		s.torrentSync = services.NewTorrentSyncService(s.db, s.integrations)
		s.eligibility = services.NewEligibilityService(s.db, s.integrations)
		s.deletion = services.NewDeletionService(
			s.db,
			s.integrations.Sonarr,
			s.integrations.Radarr,
			s.integrations.Overseerr,
			s.integrations.QBittorrent,
		)
		slog.Info("Settings updated and integrations reloaded")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Settings updated successfully",
	})
}

func (s *Server) handleTestIntegration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service  string `json:"service"`
		URL      string `json:"url"`
		APIKey   string `json:"api_key"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Service == "" || req.URL == "" {
		http.Error(w, "Service and URL are required", http.StatusBadRequest)
		return
	}

	success, message := s.testIntegrationConnection(req.Service, req.URL, req.APIKey, req.Username, req.Password)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": success,
		"message": message,
	})
}

func (s *Server) testIntegrationConnection(service string, url string, apiKey string, username string, password string) (bool, string) {
	// Create a temporary client for testing
	switch service {
	case "overseerr":
		if apiKey == "" {
			return false, "API key is required"
		}
		client := integrations.NewOverseerrClient(url, apiKey)
		_, err := client.GetRequests()
		if err != nil {
			return false, err.Error()
		}
		return true, "Connection successful"

	case "sonarr":
		if apiKey == "" {
			return false, "API key is required"
		}
		client := integrations.NewSonarrClient(url, apiKey)
		_, err := client.GetSeries()
		if err != nil {
			return false, err.Error()
		}
		return true, "Connection successful"

	case "radarr":
		if apiKey == "" {
			return false, "API key is required"
		}
		client := integrations.NewRadarrClient(url, apiKey)
		_, err := client.GetMovies()
		if err != nil {
			return false, err.Error()
		}
		return true, "Connection successful"

	case "prowlarr":
		if apiKey == "" {
			return false, "API key is required"
		}
		client := integrations.NewProwlarrClient(url, apiKey)
		_, err := client.GetIndexers()
		if err != nil {
			return false, err.Error()
		}
		return true, "Connection successful"

	case "qbittorrent":
		if username == "" || password == "" {
			return false, "Username and password are required"
		}
		client := integrations.NewQBittorrentClient(url, username, password)
		// GetTorrents will automatically login if needed
		_, err := client.GetTorrents()
		if err != nil {
			return false, err.Error()
		}
		return true, "Connection successful"

	case "tautulli":
		if apiKey == "" {
			return false, "API key is required"
		}
		client := integrations.NewTautulliClient(url, apiKey)
		_, err := client.GetHistory()
		if err != nil {
			return false, err.Error()
		}
		return true, "Connection successful"

	default:
		return false, "Unknown service"
	}
}

// handlePosterProxyRadarr proxies poster requests from Radarr
func (s *Server) handlePosterProxyRadarr(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	movieID := vars["id"]
	
	if s.integrations.Radarr == nil || !s.config.Radarr.Enabled || s.config.Radarr.URL == "" {
		http.Error(w, "Radarr not configured", http.StatusServiceUnavailable)
		return
	}
	
	// Fetch poster from Radarr
	posterURL := fmt.Sprintf("%s/MediaCover/%s/poster.jpg", s.integrations.Radarr.GetBaseURL(), movieID)
	req, err := http.NewRequest("GET", posterURL, nil)
	if err != nil {
		slog.Error("Failed to create poster request", "error", err)
		http.Error(w, "Failed to fetch poster", http.StatusInternalServerError)
		return
	}
	
	resp, err := s.integrations.Radarr.GetClient().Do(req)
	if err != nil {
		slog.Error("Failed to fetch Radarr poster", "error", err, "movie_id", movieID)
		http.Error(w, "Failed to fetch poster", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Poster not found", http.StatusNotFound)
		return
	}
	
	// Copy headers
	for k, v := range resp.Header {
		if k == "Content-Type" || k == "Content-Length" {
			w.Header()[k] = v
		}
	}
	
	// Copy body
	io.Copy(w, resp.Body)
}

// getQBittorrentStats returns statistics about tracked torrents
func (s *Server) getQBittorrentStats(ctx context.Context) map[string]interface{} {
	stats := map[string]interface{}{
		"TotalTorrents":   0,
		"SeedingTorrents": 0,
		"TotalUpload":     0,
		"TotalDownload":   0,
		"LastSync":        nil,
	}

	// Get total torrent count
	var totalCount int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM torrents").Scan(&totalCount)
	if err != nil {
		slog.Error("Failed to get torrent count", "error", err)
		return stats
	}
	stats["TotalTorrents"] = totalCount

	// Get seeding torrent count
	var seedingCount int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM torrents WHERE is_seeding = true").Scan(&seedingCount)
	if err != nil {
		slog.Error("Failed to get seeding count", "error", err)
	} else {
		stats["SeedingTorrents"] = seedingCount
	}

	// Get total upload/download bytes
	var totalUpload, totalDownload sql.NullInt64
	err = s.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(upload_bytes), 0), COALESCE(SUM(download_bytes), 0) FROM torrents",
	).Scan(&totalUpload, &totalDownload)
	if err != nil {
		slog.Error("Failed to get torrent stats", "error", err)
	} else {
		if totalUpload.Valid {
			stats["TotalUpload"] = totalUpload.Int64
		}
		if totalDownload.Valid {
			stats["TotalDownload"] = totalDownload.Int64
		}
	}

	// Get last sync time
	var lastSync sql.NullTime
	err = s.db.QueryRowContext(ctx,
		"SELECT MAX(last_synced_at) FROM torrents",
	).Scan(&lastSync)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("Failed to get last sync time", "error", err)
	} else if lastSync.Valid {
		stats["LastSync"] = lastSync.Time.Format("2006-01-02 15:04:05 MST")
	}

	return stats
}

// handlePosterProxySonarr proxies poster requests from Sonarr
func (s *Server) handlePosterProxySonarr(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	seriesID := vars["id"]
	
	if s.integrations.Sonarr == nil || !s.config.Sonarr.Enabled || s.config.Sonarr.URL == "" {
		http.Error(w, "Sonarr not configured", http.StatusServiceUnavailable)
		return
	}
	
	// Fetch poster from Sonarr
	posterURL := fmt.Sprintf("%s/MediaCover/%s/poster.jpg", s.integrations.Sonarr.GetBaseURL(), seriesID)
	req, err := http.NewRequest("GET", posterURL, nil)
	if err != nil {
		slog.Error("Failed to create poster request", "error", err)
		http.Error(w, "Failed to fetch poster", http.StatusInternalServerError)
		return
	}
	
	resp, err := s.integrations.Sonarr.GetClient().Do(req)
	if err != nil {
		slog.Error("Failed to fetch Sonarr poster", "error", err, "series_id", seriesID)
		http.Error(w, "Failed to fetch poster", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Poster not found", http.StatusNotFound)
		return
	}
	
	// Copy headers
	for k, v := range resp.Header {
		if k == "Content-Type" || k == "Content-Length" {
			w.Header()[k] = v
		}
	}
	
	// Copy body
	io.Copy(w, resp.Body)
}

