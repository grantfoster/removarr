package server

import (
	"log/slog"
	"net/http"
)

// handleLogoutPage handles GET /logout - redirects to login after clearing session
func (s *Server) handleLogoutPage(w http.ResponseWriter, r *http.Request) {
	session, err := s.store.Get(r, sessionKey)
	if err == nil {
		// Log the user_id before clearing (if it exists)
		userID := 0
		if uid, ok := session.Values[userIDKey].(int); ok {
			userID = uid
		}
		
		// Clear all session values
		for key := range session.Values {
			delete(session.Values, key)
		}
		// Set MaxAge to -1 to delete the cookie
		session.Options.MaxAge = -1
		// Save the session (this should delete the cookie)
		if err := session.Save(r, w); err != nil {
			slog.Error("Failed to save cleared session", "error", err, "user_id", userID)
		} else {
			slog.Info("Session cleared", "path", r.URL.Path, "user_id", userID)
		}
	} else {
		slog.Warn("No session found to clear", "error", err)
	}

	// Always do a regular HTTP redirect - this ensures URL updates properly
	// Don't use HTMX redirect for logout as it can cause URL/state issues
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

