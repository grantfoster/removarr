package server

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func (s *Server) handleSyncMedia(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.mediaSync.SyncAll(ctx); err != nil {
		slog.Error("Media sync failed", "error", err)
		http.Error(w, "Sync failed", http.StatusInternalServerError)
		return
	}
	if err := s.torrentSync.SyncFromQBittorrent(ctx); err != nil {
		slog.Error("Torrent sync failed", "error", err)
		// Don't fail the request, just log the error
	}

	// Redirect to refresh the dashboard
	w.Header().Set("HX-Redirect", "/dashboard")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDeleteMediaHTMX(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid media ID", http.StatusBadRequest)
		return
	}

	// Get user ID from auth context
	authCtx, ok := r.Context().Value("auth").(AuthContext)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Perform deletion
	ctx := r.Context()
	if err := s.deletion.DeleteMediaItem(ctx, id, authCtx.UserID); err != nil {
		slog.Error("Failed to delete media item", "id", id, "error", err)
		// Still remove from UI, but log the error
		// In the future, we could show an error message
	}

	// Return empty response to remove the element from the UI
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}

