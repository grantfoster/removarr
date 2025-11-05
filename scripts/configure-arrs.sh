#!/bin/bash

# Configure *arr apps with admin/admin credentials
# This waits for each service to be ready, then configures authentication

set -e

echo "🔧 Configuring *arr applications..."

# Sonarr configuration
configure_sonarr() {
    echo "   Configuring Sonarr..."
    SONARR_URL="http://localhost:8989"
    
    # Wait for Sonarr to be ready
    max_attempts=30
    attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "$SONARR_URL/api/v3/system/status" | grep -q "version"; then
            break
        fi
        attempt=$((attempt + 1))
        sleep 2
    done
    
    if [ $attempt -lt $max_attempts ]; then
        # Enable authentication and set credentials
        curl -s -X PUT "$SONARR_URL/api/v3/config/host" \
            -H "Content-Type: application/json" \
            -d '{"username":"admin","password":"admin","authentication":"forms","authenticationRequired":"enabled"}' \
            || echo "   ⚠️  Sonarr may need manual configuration"
        echo "   ✅ Sonarr: http://localhost:8989 (admin/admin)"
    fi
}

# Radarr configuration
configure_radarr() {
    echo "   Configuring Radarr..."
    RADARR_URL="http://localhost:7878"
    
    max_attempts=30
    attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "$RADARR_URL/api/v3/system/status" | grep -q "version"; then
            break
        fi
        attempt=$((attempt + 1))
        sleep 2
    done
    
    if [ $attempt -lt $max_attempts ]; then
        curl -s -X PUT "$RADARR_URL/api/v3/config/host" \
            -H "Content-Type: application/json" \
            -d '{"username":"admin","password":"admin","authentication":"forms","authenticationRequired":"enabled"}' \
            || echo "   ⚠️  Radarr may need manual configuration"
        echo "   ✅ Radarr: http://localhost:7878 (admin/admin)"
    fi
}

# Prowlarr configuration
configure_prowlarr() {
    echo "   Configuring Prowlarr..."
    PROWLARR_URL="http://localhost:9696"
    
    max_attempts=30
    attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "$PROWLARR_URL/api/v1/system/status" | grep -q "version"; then
            break
        fi
        attempt=$((attempt + 1))
        sleep 2
    done
    
    if [ $attempt -lt $max_attempts ]; then
        curl -s -X PUT "$PROWLARR_URL/api/v1/config/host" \
            -H "Content-Type: application/json" \
            -d '{"username":"admin","password":"admin","authentication":"forms","authenticationRequired":"enabled"}' \
            || echo "   ⚠️  Prowlarr may need manual configuration"
        echo "   ✅ Prowlarr: http://localhost:9696 (admin/admin)"
    fi
}

# Note: *arr apps don't support API-based auth setup on first run
# They require manual setup through the web UI
echo "⚠️  *arr applications (Sonarr, Radarr, Prowlarr) require manual authentication setup"
echo "   They don't support API-based configuration on first run."
echo ""
echo "   Please access each service and:"
echo "   1. Go to Settings → General → Security"
echo "   2. Enable authentication"
echo "   3. Set username: admin"
echo "   4. Set password: admin"
echo ""
echo "   Or use the API after first setup (see configure-arrs-api.sh)"

