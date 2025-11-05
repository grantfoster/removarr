package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type AuthContext struct {
	UserID    int
	Username  string
	IsAdmin   bool
	PlexID    *int
}

const sessionKey = "removarr_session"
const userIDKey = "user_id"
const usernameKey = "username"
const isAdminKey = "is_admin"
const plexIDKey = "plex_id"

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try Basic Auth first (for Swagger/testing)
		username, password, hasBasicAuth := r.BasicAuth()
		if hasBasicAuth {
			// Authenticate with Basic Auth
			var user struct {
				ID           int
				Username     string
				PasswordHash string
				IsAdmin      bool
				IsActive     bool
			}

			err := s.db.QueryRowContext(
				r.Context(),
				"SELECT id, username, password_hash, is_admin, is_active FROM users WHERE username = $1",
				username,
			).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.IsAdmin, &user.IsActive)

			if err == nil && user.IsActive {
				// Check password
				if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err == nil {
					// Basic Auth successful - add to context and continue
					ctx := context.WithValue(r.Context(), "auth", AuthContext{
						UserID:   user.ID,
						Username: user.Username,
						IsAdmin:  user.IsAdmin,
					})
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			// Basic Auth failed - return 401
			w.Header().Set("WWW-Authenticate", `Basic realm="Removarr"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Fall back to session-based auth
		session, err := s.store.Get(r, sessionKey)
		if err != nil {
			slog.Warn("Session error in requireAuth", "error", err, "path", r.URL.Path)
			// Redirect to login for web requests, return 401 for API
			if r.Header.Get("HX-Request") != "" || r.Header.Get("Accept") == "application/json" {
				w.Header().Set("WWW-Authenticate", `Basic realm="Removarr"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		userID, ok := session.Values[userIDKey].(int)
		if !ok || userID == 0 {
			slog.Info("No valid session found", "path", r.URL.Path, "has_session", session != nil, "userID", userID)
			// Redirect to login for web requests, return 401 for API
			if r.Header.Get("HX-Request") != "" || r.Header.Get("Accept") == "application/json" {
				w.Header().Set("WWW-Authenticate", `Basic realm="Removarr"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		
		slog.Info("Auth check passed", "user_id", userID, "path", r.URL.Path)

		// Add auth context to request
		ctx := context.WithValue(r.Context(), "auth", AuthContext{
			UserID:   userID,
			Username: session.Values[usernameKey].(string),
			IsAdmin:  session.Values[isAdminKey].(bool),
		})
		
		if plexID, ok := session.Values[plexIDKey].(int); ok && plexID > 0 {
			ctx = context.WithValue(ctx, "plex_id", plexID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check auth context first (set by requireAuth middleware)
		authCtx, ok := r.Context().Value("auth").(AuthContext)
		if ok {
			if !authCtx.IsAdmin {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Fall back to session check (for web requests)
		session, err := s.store.Get(r, sessionKey)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		isAdmin, ok := session.Values[isAdminKey].(bool)
		if !ok || !isAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// @Summary      Login
// @Description  Authenticate user and create session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        credentials  body      object  true  "Login credentials"  example({"username":"admin","password":"admin"})
// @Success      200          {object}  map[string]interface{}  "Login successful"
// @Failure      400          {object}  map[string]string  "Invalid request"
// @Failure      401          {object}  map[string]string  "Invalid credentials"
// @Router       /auth/login [post]
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get user from database
	var user struct {
		ID           int
		Username     string
		PasswordHash string
		IsAdmin      bool
		IsActive     bool
	}

	err := s.db.QueryRowContext(
		r.Context(),
		"SELECT id, username, password_hash, is_admin, is_active FROM users WHERE username = $1",
		req.Username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.IsAdmin, &user.IsActive)

	if err == sql.ErrNoRows {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if !user.IsActive {
		http.Error(w, "Account disabled", http.StatusForbidden)
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Create session
	session, err := s.store.Get(r, sessionKey)
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	session.Values[userIDKey] = user.ID
	session.Values[usernameKey] = user.Username
	session.Values[isAdminKey] = user.IsAdmin

	if err := session.Save(r, w); err != nil {
		slog.Error("Failed to save session", "error", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}
	
	slog.Info("Session saved successfully", "user_id", user.ID, "username", user.Username, "cookie_set", true)

	// Check if this is an HTMX request (from web form)
	if r.Header.Get("HX-Request") != "" {
		// HTMX request - redirect via HX-Redirect header
		// This will cause HTMX to do a full page navigation
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Redirecting..."))
		slog.Info("HTMX login redirect", "redirect_to", "/dashboard")
		return
	}
	
	// Check if this is a regular form submission (not JSON)
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/json" {
		// Regular form submission - redirect
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		slog.Info("Form login redirect", "redirect_to", "/dashboard")
		return
	}

	// JSON API request
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"user": map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"is_admin": user.IsAdmin,
		},
	})
}

// @Summary      Logout
// @Description  Logout and destroy session
// @Tags         auth
// @Produce      json
// @Success      200  {object}  map[string]bool  "Logout successful"
// @Router       /auth/logout [post]
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	session, err := s.store.Get(r, sessionKey)
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	session.Values = make(map[interface{}]interface{})
	session.Options.MaxAge = -1

	if err := session.Save(r, w); err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	// Check if HTMX request
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handlePlexAuth(w http.ResponseWriter, r *http.Request) {
	if s.integrations.Plex == nil {
		http.Error(w, "Plex integration not enabled", http.StatusBadRequest)
		return
	}

	// TODO: Implement Plex OAuth flow
	// This is a placeholder - Plex OAuth requires:
	// 1. Client ID from Plex
	// 2. OAuth flow with redirect
	// 3. Token exchange
	// 4. User lookup

	http.Error(w, "Plex authentication not yet implemented", http.StatusNotImplemented)
}

