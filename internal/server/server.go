package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"removarr/internal/config"
	"removarr/internal/integrations"
	"removarr/internal/services"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	httpSwagger "github.com/swaggo/http-swagger"
)

type Server struct {
	config         *config.Config
	configPath     string // Path to config file for persistence
	db             *sql.DB
	router         *mux.Router
	httpServer     *http.Server
	integrations   *integrations.Client
	store          *sessions.CookieStore
	mediaSync      *services.MediaSyncService
	torrentSync    *services.TorrentSyncService
	eligibility    *services.EligibilityService
	deletion       *services.DeletionService
}

func New(cfg *config.Config, db *sql.DB, configPath string) *Server {
	router := mux.NewRouter()
	
	// Create session store
	store := sessions.NewCookieStore([]byte(cfg.Server.SessionSecret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   int(cfg.Server.SessionMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		// Don't set Domain - let it default so it works on localhost
	}

	// Create integrations client
	integrationsClient := integrations.NewClient(cfg)

	// Create services
	mediaSyncService := services.NewMediaSyncService(db, integrationsClient)
	torrentSyncService := services.NewTorrentSyncService(db, integrationsClient)
	eligibilityService := services.NewEligibilityService(db, integrationsClient)
	deletionService := services.NewDeletionService(
		db,
		integrationsClient.Sonarr,
		integrationsClient.Radarr,
		integrationsClient.Overseerr,
		integrationsClient.QBittorrent,
	)

	srv := &Server{
		config:       cfg,
		configPath:   configPath,
		db:           db,
		router:       router,
		integrations: integrationsClient,
		store:        store,
		mediaSync:    mediaSyncService,
		torrentSync:  torrentSyncService,
		eligibility:  eligibilityService,
		deletion:     deletionService,
	}

	// Initialize templates
	if err := initTemplates(); err != nil {
		slog.Error("Failed to initialize templates", "error", err)
		// Continue anyway - templates will fail gracefully
	}

	srv.setupRoutes()

	// Load settings from database and merge with config
	srv.loadIntegrationSettings()
	
	// Reload integrations with merged config
	integrationsClient = integrations.NewClient(srv.config)
	srv.integrations = integrationsClient
	srv.mediaSync = services.NewMediaSyncService(db, integrationsClient)
	srv.torrentSync = services.NewTorrentSyncService(db, integrationsClient)
	srv.eligibility = services.NewEligibilityService(db, integrationsClient)
	srv.deletion = services.NewDeletionService(
		db,
		integrationsClient.Sonarr,
		integrationsClient.Radarr,
		integrationsClient.Overseerr,
		integrationsClient.QBittorrent,
	)

	srv.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start periodic sync goroutine
	go srv.startPeriodicSync()

	return srv
}

// startPeriodicSync runs a background goroutine that syncs at a configurable interval
func (s *Server) startPeriodicSync() {
	var ticker *time.Ticker
	var currentFrequency time.Duration = 5 * time.Minute // Default
	
	// Initial ticker
	ticker = time.NewTicker(currentFrequency)
	defer ticker.Stop()
	
	// Check for frequency changes periodically
	frequencyCheck := time.NewTicker(1 * time.Minute)
	defer frequencyCheck.Stop()

	for {
		select {
		case <-ticker.C:
			slog.Info("Starting periodic sync", "frequency", currentFrequency)
			ctx := context.Background()
			if err := s.mediaSync.SyncAll(ctx); err != nil {
				slog.Error("Periodic sync failed", "error", err)
			} else {
				slog.Info("Periodic sync completed successfully")
			}
			// Also sync torrents
			if err := s.torrentSync.SyncFromQBittorrent(ctx); err != nil {
				slog.Error("Periodic torrent sync failed", "error", err)
			}
		case <-frequencyCheck.C:
			// Check if frequency changed
			var syncFrequencyStr string
			err := s.db.QueryRowContext(context.Background(),
				"SELECT value FROM settings WHERE key = 'sync_frequency'",
			).Scan(&syncFrequencyStr)

			var newFrequency time.Duration = 5 * time.Minute // Default
			if err == nil && syncFrequencyStr != "" {
				if parsed, err := time.ParseDuration(syncFrequencyStr); err == nil {
					newFrequency = parsed
				}
			}
			
			// Update ticker if frequency changed
			if newFrequency != currentFrequency {
				slog.Info("Sync frequency changed, updating ticker", "old", currentFrequency, "new", newFrequency)
				ticker.Stop()
				currentFrequency = newFrequency
				ticker = time.NewTicker(currentFrequency)
			}
		}
	}
}

func (s *Server) setupRoutes() {
	// Static files
	s.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))))

	// Swagger UI
	s.router.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"), // The url pointing to API definition
	))

	// Health check
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// Setup wizard (check if setup is needed)
	s.router.HandleFunc("/setup", s.handleSetup).Methods("GET", "POST")

	// API routes
	api := s.router.PathPrefix("/api").Subrouter()
	
	// Auth routes
	api.HandleFunc("/auth/login", s.handleLogin).Methods("POST")
	api.HandleFunc("/auth/logout", s.handleLogout).Methods("POST")
	api.HandleFunc("/auth/plex", s.handlePlexAuth).Methods("GET", "POST")

	// Protected routes
	protected := api.PathPrefix("").Subrouter()
	protected.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(s.requireAuth(next.ServeHTTP))
	})
	
	protected.HandleFunc("/media", s.handleListMedia).Methods("GET")
	protected.HandleFunc("/media/{id}/delete", s.handleDeleteMedia).Methods("POST")
	protected.HandleFunc("/media/bulk-delete", s.handleBulkDeleteMedia).Methods("POST")

	// Admin routes
	admin := protected.PathPrefix("/admin").Subrouter()
	admin.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(s.requireAdmin(next.ServeHTTP))
	})
	
	admin.HandleFunc("/users", s.handleListUsers).Methods("GET")
	admin.HandleFunc("/users", s.handleCreateUser).Methods("POST")
	admin.HandleFunc("/users/{id}", s.handleUpdateUser).Methods("PUT")
	admin.HandleFunc("/users/{id}", s.handleDeleteUser).Methods("DELETE")
	admin.HandleFunc("/users/import-plex", s.handleImportPlexUsers).Methods("POST")
	admin.HandleFunc("/settings", s.handleGetSettings).Methods("GET")
	admin.HandleFunc("/settings", s.handleUpdateSettings).Methods("PUT")
	admin.HandleFunc("/settings/test", s.handleTestIntegration).Methods("POST")

	// Public web routes
	s.router.HandleFunc("/", s.handleIndex).Methods("GET")
	s.router.HandleFunc("/login", s.handleLoginPage).Methods("GET")
	s.router.HandleFunc("/logout", s.handleLogoutPage).Methods("GET")
	
	// Protected web routes
	protectedWeb := s.router.PathPrefix("").Subrouter()
	protectedWeb.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for login page, root, and static assets
			if r.URL.Path == "/login" || r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/static/") || strings.HasPrefix(r.URL.Path, "/swagger/") || r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			s.requireAuth(next.ServeHTTP)(w, r)
		})
	})
	protectedWeb.HandleFunc("/dashboard", s.handleDashboard).Methods("GET")
	protectedWeb.HandleFunc("/admin", s.handleAdminPage).Methods("GET")
	protectedWeb.HandleFunc("/admin/settings", s.handleSettingsPage).Methods("GET")
	
	// HTMX endpoints (protected)
	protectedWeb.HandleFunc("/api/media/sync", s.handleSyncMedia).Methods("POST")
	protectedWeb.HandleFunc("/api/media/{id}", s.handleDeleteMediaHTMX).Methods("DELETE")
}

func (s *Server) Start() error {
	slog.Info("Server starting", "address", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

