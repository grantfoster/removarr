package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"removarr/internal/config"
	"removarr/internal/server"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "removarr/docs" // swag init will generate this
)

// @title           Removarr API
// @version         1.0
// @description     API for managing seedbox media deletion
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.example.com/support

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /api

// @securityDefinitions.basic  BasicAuth
func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		// Try to use defaults if config doesn't exist (for setup wizard)
		cfg = config.Default()
		slog.Info("Config file not found, using defaults. Setup wizard will be available.")
	}

	// Setup logger
	logger := setupLogger(cfg)
	slog.SetDefault(logger)

	// Connect to database
	db, err := connectDatabase(cfg)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create server
	srv := server.New(cfg, db, configPath)

	// Start server
	go func() {
		slog.Info("Starting server", "host", cfg.Server.Host, "port", cfg.Server.Port)
		if err := srv.Start(); err != nil {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")
	if err := srv.Shutdown(context.Background()); err != nil {
		slog.Error("Error shutting down server", "error", err)
	}
}

func setupLogger(cfg *config.Config) *slog.Logger {
	var opts *slog.HandlerOptions
	switch cfg.Logging.Level {
	case "debug":
		opts = &slog.HandlerOptions{Level: slog.LevelDebug}
	case "info":
		opts = &slog.HandlerOptions{Level: slog.LevelInfo}
	case "warn":
		opts = &slog.HandlerOptions{Level: slog.LevelWarn}
	case "error":
		opts = &slog.HandlerOptions{Level: slog.LevelError}
	default:
		opts = &slog.HandlerOptions{Level: slog.LevelInfo}
	}

	var handler slog.Handler
	if cfg.Logging.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func connectDatabase(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Database,
		cfg.Database.SSLMode,
	)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

