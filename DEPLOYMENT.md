# Removarr Seedbox Deployment Guide

This guide helps you deploy Removarr to a seedbox without root access.

## Prerequisites

1. **Go installed** (for building) OR **pre-built binary** (recommended)
2. **PostgreSQL access** - either:
   - Existing PostgreSQL database you can access
   - Or PostgreSQL on your seedbox (some providers offer this)
3. **Network access** to your *arr apps (Overseerr, Sonarr, Radarr, etc.)

## Option 1: Quick Deploy (Pre-built Binary)

### Step 1: Download Binary

```bash
# Create directory
mkdir -p ~/removarr
cd ~/removarr

# Download latest release (replace with your GitHub repo URL when published)
# For now, build from source (see Option 2)
```

### Step 2: Create Config File

```bash
# Copy example config
cp config.example.yaml config.yaml

# Edit with your database connection
nano config.yaml
```

Minimal config needed:
```yaml
server:
  host: "0.0.0.0"
  port: 8080
  base_url: "http://your-seedbox-ip:8080"
  session_secret: "" # Will be auto-generated or set via env var

database:
  host: "localhost"  # Or your PostgreSQL host
  port: 5432
  user: "your_db_user"
  password: "your_db_password"
  database: "removarr"
  ssl_mode: "disable"  # Or "require" if your provider needs it

logging:
  level: "info"
  format: "json"
```

### Step 3: Create Database

```bash
# Connect to PostgreSQL and create database
psql -h your_host -U your_user -d postgres
CREATE DATABASE removarr;
\q
```

### Step 4: Run Migrations

```bash
# Using Go command
go run ./cmd/migrate -config config.yaml -cmd up

# Or build migrate binary first
go build -o migrate ./cmd/migrate
./migrate -config config.yaml -cmd up
```

### Step 5: Run Removarr

```bash
# Build binary
go build -o removarr ./cmd/removarr

# Run in screen/tmux
screen -dmS removarr ./removarr -config config.yaml

# Or run in background
nohup ./removarr -config config.yaml > removarr.log 2>&1 &
```

### Step 6: Access Removarr

1. Open `http://your-seedbox-ip:8080` in your browser
2. Complete the setup wizard (create admin account)
3. Go to Settings and configure your integrations

## Option 2: Build from Source

### Step 1: Clone and Build

```bash
# Clone repository
git clone https://github.com/your-username/removarr.git
cd removarr

# Build binary
go build -o removarr ./cmd/removarr

# Or build migrate tool
go build -o migrate ./cmd/migrate
```

### Step 2: Continue with Steps 2-6 from Option 1

## Running as a Service (No Root)

### Using Screen (Simple)

```bash
# Create screen session
screen -dmS removarr ~/removarr/removarr -config ~/removarr/config.yaml

# Attach to screen
screen -r removarr

# Detach: Ctrl+A then D

# Auto-start on reboot (add to ~/.bashrc or ~/.profile)
echo 'screen -dmS removarr ~/removarr/removarr -config ~/removarr/config.yaml' >> ~/.bashrc
```

### Using Systemd User Service (If Available)

Create `~/.config/systemd/user/removarr.service`:

```ini
[Unit]
Description=Removarr Media Management
After=network.target

[Service]
Type=simple
WorkingDirectory=%h/removarr
ExecStart=%h/removarr/removarr -config %h/removarr/config.yaml
Restart=always
RestartSec=10

[Install]
WantedBy=default.target
```

Then:
```bash
systemctl --user enable removarr
systemctl --user start removarr
```

## Port Configuration

If port 8080 is blocked, change it in `config.yaml`:
```yaml
server:
  port: 8081  # Use any available port
```

## Database Options

### Option A: Shared PostgreSQL (Most Seedboxes)

Many seedbox providers offer PostgreSQL. Use their connection details.

### Option B: Docker PostgreSQL (If Docker Available)

```bash
docker run -d \
  --name removarr-db \
  -e POSTGRES_USER=removarr \
  -e POSTGRES_PASSWORD=your_password \
  -e POSTGRES_DB=removarr \
  -p 127.0.0.1:5432:5432 \
  postgres:15
```

### Option C: SQLite (Not Currently Supported)

Would require code changes to support SQLite.

## Troubleshooting

### Can't Connect to Database

- Check if PostgreSQL is accessible from your user account
- Verify host, port, username, password in config.yaml
- Test connection: `psql -h host -U user -d database`

### Port Already in Use

- Change port in config.yaml
- Check what's using the port: `netstat -tulpn | grep 8080`

### Migrations Fail

- Check database connection
- Verify database exists: `psql -h host -U user -l`
- Check migration version: `go run ./cmd/migrate -config config.yaml -cmd version`

### Can't Access Web UI

- Check if service is running: `screen -ls` or `ps aux | grep removarr`
- Check firewall/port forwarding
- Verify `base_url` in config matches your access method

## Updating

```bash
cd ~/removarr
git pull
go build -o removarr ./cmd/removarr
# Restart service (screen or systemd)
```

## Logs

Logs go to stdout by default. If running with nohup:
```bash
tail -f removarr.log
```

Or if running in screen:
```bash
screen -r removarr
# View logs in terminal
```

## Security Notes

1. **Session Secret**: Set `REMOVARR_SESSION_SECRET` env var or in config.yaml
2. **Database Password**: Use env var `REMOVARR_DB_PASSWORD` instead of config file
3. **Firewall**: Only expose port 8080 if needed (use reverse proxy if possible)
4. **HTTPS**: Use a reverse proxy (nginx, caddy) for HTTPS in production

