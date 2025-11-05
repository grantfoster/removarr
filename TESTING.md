# Testing Removarr Locally

This guide will help you test Removarr on your local machine without needing a full production seedbox setup.

## Quick Start

### Option 1: Docker Compose (Recommended)

1. **Start all the arr services:**
   ```bash
   ./scripts/setup-test-env.sh
   ```

   This will:
   - Start all Docker services
   - Wait for PostgreSQL to be ready
   - Run database migrations
   - Configure qBittorrent (admin/admin)
   - Create config.yaml if needed

2. **Configure *arr apps:**
   - The setup script will guide you, but here's what you need to do:
   - **Sonarr**: http://localhost:8989 → Settings → General → Security → Enable auth → Set username: `admin`, password: `admin`
   - **Radarr**: http://localhost:7878 → Settings → General → Security → Enable auth → Set username: `admin`, password: `admin`
   - **Prowlarr**: http://localhost:9696 → Settings → General → Security → Enable auth → Set username: `admin`, password: `admin`
   - **Overseerr**: http://localhost:5055 → Complete setup wizard (first time only)

3. **Get API keys:**
   - After enabling auth, get API keys from Settings → General in each service
   - Edit `config.yaml` with your API keys
   - **Important**: API keys persist in Docker volumes, so you only need to set them once!

4. **Run Removarr:**
   ```bash
   go run ./cmd/removarr
   ```

### Option 2: Use Existing Services

If you already have arr apps running locally or on a server:

1. **Point your config to existing services**
2. **Run migrations on your database**
3. **Start Removarr**

## API Keys Persistence

**Good news**: API keys persist across Docker restarts!

- **`docker-compose down`** - Stops containers, **keeps volumes** (API keys preserved ✅)
- **`docker-compose stop`** - Stops containers, **keeps everything** (API keys preserved ✅)
- **`docker-compose down -v`** - Removes volumes (API keys reset ❌)

**You only need to update API keys if:**
- You use `docker-compose down -v` (removes volumes)
- You delete the Docker volumes manually
- You're setting up for the first time

**To check service status:**
```bash
./scripts/fetch-api-keys.sh
```

## Authentication Setup

### qBittorrent

**Important**: qBittorrent generates a random password on first run. The setup script will automatically fix this.

- **After setup**: admin/admin
- **If you see 401 errors:**
  1. Run the fix script: `./scripts/fix-qbittorrent-webui.sh`
  2. Or check logs for temp password: `docker logs removarr-test-qbittorrent | grep -i password`
  3. Clear browser cache/cookies
  4. Try incognito/private mode

### *arr Apps (Sonarr, Radarr, Prowlarr)
- **No default auth** - accessible without authentication initially
- **Must configure manually** on first run:
  1. Access the service
  2. Go to Settings → General → Security
  3. Enable authentication
  4. Set username: `admin`
  5. Set password: `admin`
  6. Save settings

### Overseerr
- **Setup wizard** on first access
- Follow the on-screen instructions

## Access URLs

- **Removarr**: http://localhost:8080 (when running)
- **Swagger UI**: http://localhost:8080/swagger/index.html
- **Sonarr**: http://localhost:8989
- **Radarr**: http://localhost:7878
- **Prowlarr**: http://localhost:9696
- **Overseerr**: http://localhost:5055
- **qBittorrent**: http://localhost:8081

## Using the API

### 1. Create Admin User

First time setup:
```bash
./scripts/create-admin-user.sh admin admin
```

### 2. Test Endpoints in Swagger

1. Open http://localhost:8080/swagger/index.html
2. Click "Authorize" button
3. Enter: username `admin`, password `admin`
4. Try endpoints:
   - `GET /api/media?sync=true` - Sync and list media
   - `GET /api/media` - List cached media
   - `GET /api/admin/users` - List users

### 3. Sync Media

The `/api/media` endpoint supports:
- `?sync=true` - Sync from Sonarr/Radarr before listing
- `?type=movie` or `?type=series` - Filter by type
- `?user_id=1` - Filter by user

## Troubleshooting qBittorrent

### If you see "Unauthorized" with no login form:

1. **Run the fix script:**
   ```bash
   ./scripts/fix-qbittorrent-webui.sh
   ```
   This will:
   - Stop qBittorrent
   - Modify the config file to set admin/admin
   - Restart qBittorrent

2. **Or manually check the password:**
   ```bash
   docker logs removarr-test-qbittorrent | grep -i "temporary password"
   ```
   Use that password to login, then change it in settings.

3. **Clear browser cache:**
   - Clear all cookies for localhost:8081
   - Try incognito/private mode

4. **Check if qBittorrent is fully started:**
   ```bash
   docker logs removarr-test-qbittorrent
   ```
   Wait until you see "WebUI will be started shortly after internal preparations. Please wait..."

## Setting Up Test Data

### For Sonarr/Radarr:

1. **Add test media manually:**
   - Go to Sonarr/Radarr web UI
   - Add a test series/movie
   - You can use fake/test data or add real media metadata (without downloading)

2. **Create test files:**
   ```bash
   mkdir -p test-data/media/test-series
   touch test-data/media/test-series/episode1.mkv
   # Add some size to the file
   truncate -s 1G test-data/media/test-series/episode1.mkv
   ```

### For Prowlarr:

1. **Add test indexers:**
   - You can use public trackers for testing (like RARBG, 1337x, etc.)
   - Or use Prowlarr's built-in test indexers
   - For private trackers, you'd need actual credentials (but you can test without adding real trackers)

2. **Configure seeding requirements:**
   - In Prowlarr, set minimum seeding time/ratio for test trackers
   - Public trackers typically don't require seeding (good for testing deletion logic)

### For Overseerr:

1. **Set up Overseerr:**
   - Connect it to Sonarr/Radarr
   - Connect it to Plex (optional, or skip for testing)
   - Create a test user account

2. **Make test requests:**
   - Request some media through Overseerr
   - This will create requests that Removarr can track

### For qBittorrent:

1. **Login:** admin/admin (after auto-config)
2. **Add test torrents:**
   - You can add test torrent files (legal test torrents)
   - Or configure it to connect to Sonarr/Radarr automatically

## Legal Testing Options

### Public Trackers (Legal for Testing):

- **1337x** - Public tracker (don't download copyrighted content, just test the integration)
- **RARBG** - Public tracker (same note)
- **ThePirateBay** - Public tracker

**Note:** You're only testing the *integration* - you don't need to actually download copyrighted content. You can:
- Add the tracker to Prowlarr
- Test the API connections
- Test the seeding logic with mock data
- Use legal test torrents (like Linux ISOs)

### Test Trackers:

- Use **Prowlarr's test indexers** (built-in)
- Create **mock indexers** in Prowlarr for testing

### Legal Test Torrents:

- Linux distributions (Ubuntu, Debian, etc.)
- Open source software
- Creative Commons content
- Test files from trackers that allow them

## Testing Workflow

### 1. Test Database Setup

```bash
# Create database (handled by setup script)
# Or manually:
createdb removarr

# Run migrations (handled by setup script)
# Or manually:
docker run --rm --network removarr_default \
  -v "$(pwd)/migrations:/migrations" \
  migrate/migrate:v4.19.0 \
  -path /migrations \
  -database "postgres://removarr:removarr@postgres:5432/removarr?sslmode=disable" up
```

### 2. Test Configuration

Create `config.yaml`:
```yaml
server:
  host: "0.0.0.0"
  port: 8080

database:
  host: "localhost"
  port: 5433  # Note: Using 5433 to avoid conflict with host PostgreSQL
  user: "removarr"
  password: "removarr"
  database: "removarr"
  ssl_mode: "disable"

sonarr:
  enabled: true
  url: "http://localhost:8989"
  api_key: "YOUR_SONARR_API_KEY"  # Get from Sonarr Settings > General

radarr:
  enabled: true
  url: "http://localhost:7878"
  api_key: "YOUR_RADARR_API_KEY"  # Get from Radarr Settings > General

prowlarr:
  enabled: true
  url: "http://localhost:9696"
  api_key: "YOUR_PROWLARR_API_KEY"  # Get from Prowlarr Settings > General

overseerr:
  enabled: true
  url: "http://localhost:5055"
  api_key: "YOUR_OVERSEERR_API_KEY"  # Get from Overseerr Settings > General

qbittorrent:
  enabled: true
  url: "http://localhost:8081"
  username: "admin"
  password: "admin"
```

### 3. Test Scenarios

#### Test 1: Media Sync
- Add media to Sonarr/Radarr
- Run: `curl -u admin:admin "http://localhost:8080/api/media?sync=true"`
- Check if media appears in response

#### Test 2: Seeding Requirements
- Add a tracker to Prowlarr with seeding requirements
- Add a torrent in qBittorrent
- Sync: `curl -u admin:admin "http://localhost:8080/api/media?sync=true"`
- Check if eligibility is correctly determined

#### Test 3: Deletion Workflow
- Mark media as eligible for deletion
- Click delete
- Verify deletion in all services

#### Test 4: User Management
- Create a test user
- Import Plex users (if Plex is set up)
- Test permissions

## Troubleshooting

### Services won't start:
```bash
docker-compose -f docker-compose.test.yml logs
```

### Database connection issues:
- Check PostgreSQL is running: `docker ps`
- Check connection: `docker exec removarr-test-db psql -U removarr -d removarr -c "SELECT 1"`

### API key issues:
- Get API keys from each service's Settings > General page
- Make sure services are fully started before getting keys
- Make sure authentication is enabled in *arr apps before getting API keys
- **Remember**: API keys persist in volumes unless you use `-v` flag

### Port conflicts:
- Change ports in docker-compose.test.yml if you have existing services
- PostgreSQL uses 5433 to avoid conflict with host PostgreSQL

## Stopping Test Services

```bash
# Stop but keep volumes (API keys preserved)
docker-compose -f docker-compose.test.yml down

# Or just stop (keeps everything)
docker-compose -f docker-compose.test.yml stop

# Remove volumes (clean slate, API keys reset)
docker-compose -f docker-compose.test.yml down -v
```

## Deletion Flow Testing

Since it's difficult to get real media files for testing, here are several approaches:

### Option 1: Automated Test Script

Run the automated test script:

```bash
./scripts/test-deletion-flow.sh
```

This script will:
1. Add a test movie to Radarr (using TMDB metadata)
2. Sync it to Removarr
3. Test the deletion flow
4. Verify deletion in all services

### Option 2: Manual Testing (No Real Files)

See [scripts/test-deletion-manual.md](scripts/test-deletion-manual.md) for detailed manual testing steps.

**Quick summary:**
1. Add a movie to Radarr (monitored, but don't download)
2. Sync in Removarr dashboard
3. Delete via Removarr UI
4. Verify deletion in all services

### Option 3: Test Individual Components

You can test each integration separately:

**Test Radarr deletion:**
```bash
curl -X DELETE "http://localhost:7878/api/v3/movie/1" \
  -H "X-Api-Key: YOUR_RADARR_API_KEY"
```

**Test Overseerr deletion:**
```bash
curl -X DELETE "http://localhost:5055/api/v1/request/1" \
  -H "X-Api-Key: YOUR_OVERSEERR_API_KEY"
```

**Test qBittorrent deletion:**
```bash
curl -X GET "http://localhost:8081/api/v2/torrents/delete?hashes=HASH&deleteFiles=true" \
  -b /tmp/qb-cookies.txt
```

### Verification Checklist

After deletion, verify:
- [ ] Media item removed from Removarr dashboard
- [ ] Movie unmonitored or deleted in Radarr
- [ ] Request deleted in Overseerr (if applicable)
- [ ] Torrent removed from qBittorrent (if applicable)
- [ ] Files deleted from filesystem (if downloaded)
- [ ] Audit log entry created in Removarr database
- [ ] No errors in `/tmp/removarr.log`

**Check audit log:**
```bash
psql -h localhost -p 5433 -U removarr -d removarr -c \
  "SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT 5;"
```
