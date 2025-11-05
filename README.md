# Removarr

A Golang/HTMX application for managing seedbox storage by allowing users to delete media that has been seeding long enough.

## Features

- Integration with Overseerr, Sonarr, Radarr, Prowlarr, qBittorrent, and Tautulli
- Smart seeding requirement detection via Prowlarr
- Plex authentication + local user/password
- Multi-user support with admin panel
- Media deletion with confirmation workflow
- Audit logging for deletions
- Beautiful UI with Tailwind CSS

## Installation

### Seedbox Installation (No Root Required)

**Quick Deploy:**
```bash
# From your project directory
./scripts/deploy-seedbox.sh
```

**Manual Steps:**
1. Build binary: `go build -o removarr ./cmd/removarr`
2. Create config.yaml (copy from config.example.yaml)
3. Run migrations: `go run ./cmd/migrate -config config.yaml -cmd up`
4. Start: `screen -dmS removarr ./removarr -config config.yaml`

See [DEPLOYMENT.md](DEPLOYMENT.md) for detailed seedbox deployment guide.

### Podman/Docker

```bash
podman build -t removarr .
podman run -d --name removarr -p 8080:8080 removarr
```

## Configuration

Removarr uses a configuration file (YAML) and a web UI for settings management. See `config.example.yaml` for available options.

## Development

### Local Testing

See [TESTING.md](TESTING.md) for a comprehensive guide on testing Removarr locally.

Quick start:
```bash
# Start test services with Docker Compose
docker-compose -f docker-compose.test.yml up -d

# Run migrations
migrate -path migrations -database "postgres://removarr:removarr@localhost:5432/removarr?sslmode=disable" up

# Run the application
go run ./cmd/removarr
```

### Building

```bash
# Build binary
make build

# Or manually
go build -o removarr ./cmd/removarr
```

## License

MIT

