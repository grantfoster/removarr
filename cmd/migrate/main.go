package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"removarr/internal/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var configPath string
	var command string

	flag.StringVar(&configPath, "config", "config.yaml", "Path to configuration file")
	flag.StringVar(&command, "cmd", "", "Command: up, down, reset, or version")
	flag.Parse()

	if command == "" {
		flag.Usage()
		fmt.Println("\nCommands:")
		fmt.Println("  up      Apply all pending migrations")
		fmt.Println("  down    Roll back the last migration")
		fmt.Println("  reset   Drop everything and re-run all migrations")
		fmt.Println("  version Print current migration version")
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Build database URL
	dbURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
		cfg.Database.SSLMode,
	)

	// Create migrate instance
	m, err := migrate.New("file://migrations", dbURL)
	if err != nil {
		log.Fatalf("Failed to create migrate instance: %v", err)
	}
	defer m.Close()

	// Handle dirty state
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		log.Fatalf("Failed to get version: %v", err)
	}
	if dirty {
		fmt.Printf("⚠️  Database is dirty at version %d. Fixing...\n", version)
		if err := m.Force(int(version)); err != nil {
			log.Fatalf("Failed to force version: %v", err)
		}
		fmt.Println("✅ Dirty state cleared")
	}

	// Execute command
	switch command {
	case "up":
		fmt.Println("🔼 Running migrations up...")
		if err := m.Up(); err != nil {
			if err == migrate.ErrNoChange {
				fmt.Println("✅ Database is already up to date")
				return
			}
			log.Fatalf("Migration failed: %v", err)
		}
		version, _, _ := m.Version()
		fmt.Printf("✅ Migrations completed! Current version: %d\n", version)

	case "down":
		fmt.Println("🔽 Rolling back last migration...")
		if err := m.Down(); err != nil {
			if err == migrate.ErrNoChange {
				fmt.Println("✅ No migrations to roll back")
				return
			}
			log.Fatalf("Rollback failed: %v", err)
		}
		version, _, _ := m.Version()
		fmt.Printf("✅ Rollback completed! Current version: %d\n", version)

	case "reset":
		fmt.Println("🔄 Resetting database (this will drop all tables!)...")
		if err := m.Drop(); err != nil {
			log.Fatalf("Drop failed: %v", err)
		}
		fmt.Println("✅ Database dropped")
		fmt.Println("🔼 Running all migrations...")
		if err := m.Up(); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		version, _, _ := m.Version()
		fmt.Printf("✅ Reset complete! Current version: %d\n", version)

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			if err == migrate.ErrNilVersion {
				fmt.Println("Database version: 0 (no migrations applied)")
				return
			}
			log.Fatalf("Failed to get version: %v", err)
		}
		if dirty {
			fmt.Printf("Database version: %d (DIRTY)\n", version)
		} else {
			fmt.Printf("Database version: %d\n", version)
		}

	default:
		log.Fatalf("Unknown command: %s. Use: up, down, reset, or version", command)
	}
}

