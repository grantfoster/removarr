#!/bin/bash

# Quick setup script for local testing

set -e

echo "🚀 Setting up Removarr test environment..."

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker and try again."
    exit 1
fi

# Start services
echo "📦 Starting Docker services..."
docker-compose -f docker-compose.test.yml up -d

# Wait for PostgreSQL to be ready
echo "⏳ Waiting for PostgreSQL to be ready..."
max_attempts=30
attempt=0

while [ $attempt -lt $max_attempts ]; do
    if docker exec removarr-test-db pg_isready -U removarr > /dev/null 2>&1; then
        # Try to connect to verify the database exists
        if docker exec removarr-test-db psql -U removarr -d removarr -c "SELECT 1" > /dev/null 2>&1; then
            echo "✅ PostgreSQL is ready!"
            break
        fi
    fi
    attempt=$((attempt + 1))
    echo "   Waiting... ($attempt/$max_attempts)"
    sleep 2
done

if [ $attempt -eq $max_attempts ]; then
    echo "❌ PostgreSQL failed to become ready after $max_attempts attempts"
    echo "   Check logs: docker logs removarr-test-db"
    exit 1
fi

# Run migrations using Go command
echo "🗄️  Running database migrations..."

if go run ./cmd/migrate -config config.yaml -cmd up; then
    echo "✅ Migrations completed!"
else
    echo "❌ Migrations failed."
    echo "   Make sure config.yaml has correct database settings."
    exit 1
fi

# Create config file if it doesn't exist
if [ ! -f "config.yaml" ]; then
    echo "📝 Creating config.yaml from example..."
    cp config.example.yaml config.yaml
    echo "✅ Created config.yaml"
    echo ""
    echo "   Note: Integration settings (API keys, URLs) are now managed"
    echo "   through the web UI and stored in the database."
    echo "   The config file only needs database connection settings."
else
    echo "✅ config.yaml already exists"
fi

# Configure qBittorrent
echo ""
echo "🔧 Configuring qBittorrent..."
echo "   qBittorrent generates a random password on first run."
echo "   Resetting to get a fresh temporary password..."

if [ -f "scripts/reset-qbittorrent-auth.sh" ]; then
    bash scripts/reset-qbittorrent-auth.sh
    
    # Try to set password to admin via API
    echo ""
    echo "   Attempting to set password to 'admin'..."
    sleep 3
    
    # Get the temp password from logs
    TEMP_PASS=$(docker logs removarr-test-qbittorrent 2>&1 | grep -i "temporary password" | tail -1 | sed -n 's/.*: \([^ ]*\)/\1/p' || echo "")
    
    if [ -n "$TEMP_PASS" ]; then
        # Login with temp password
        RESPONSE=$(curl -s -c /tmp/qb_setup_cookies.txt -X POST "http://localhost:8081/api/v2/auth/login" \
            -d "username=admin&password=$TEMP_PASS" 2>/dev/null || echo "Failed")
        
        if [ "$RESPONSE" = "Ok." ]; then
            # Set password to admin
            curl -s -b /tmp/qb_setup_cookies.txt -X POST "http://localhost:8081/api/v2/app/setPreferences" \
                -d "json={\"web_ui_password\":\"admin\"}" > /dev/null 2>&1
            curl -s -b /tmp/qb_setup_cookies.txt -X POST "http://localhost:8081/api/v2/auth/logout" > /dev/null 2>&1
            echo "   ✅ Password set to 'admin'!"
        else
            echo "   ⚠️  Could not set password automatically"
            echo "   Use temp password: $TEMP_PASS"
            echo "   Then set password to 'admin' manually in Web UI"
        fi
    else
        echo "   ⚠️  Could not find temp password in logs"
        echo "   Check: docker logs removarr-test-qbittorrent | grep -i password"
    fi
else
    echo "   ⚠️  Reset script not found"
fi

echo ""
echo "✅ Setup complete!"
echo ""
echo "📋 Services are running at:"
echo "   - Removarr: http://localhost:8080 (when you start it)"
echo "   - Sonarr: http://localhost:8989 (no auth initially - configure in Settings → General → Security)"
echo "   - Radarr: http://localhost:7878 (no auth initially - configure in Settings → General → Security)"
echo "   - Prowlarr: http://localhost:9696 (no auth initially - configure in Settings → General → Security)"
echo "   - Overseerr: http://localhost:5055 (setup wizard on first access)"
echo "   - qBittorrent: http://localhost:8081 (admin/admin)"
echo ""
echo "▶️  Next steps:"
echo "   1. Run: go run ./cmd/removarr"
echo "   2. Access Removarr at http://localhost:8080"
echo "   3. Complete the setup wizard (create admin account)"
echo "   4. Go to Settings and configure integrations:"
echo "      - Access each *arr service and enable authentication (admin/admin)"
echo "      - Get API keys from Settings → General in each service"
echo "      - Add API keys in Removarr Settings page"
echo ""
echo "💡 Note: *arr apps don't have default auth - you must set it up manually"
echo "   on first run. They'll be accessible without auth initially."
echo "💡 Integration settings are stored in the database, not config.yaml"
echo ""
echo "🛑 To stop services: docker-compose -f docker-compose.test.yml down"

