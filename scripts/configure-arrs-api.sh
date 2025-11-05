#!/bin/bash

# Configure *arr apps with admin/admin AFTER initial setup
# This requires the apps to already be configured (no auth required initially)

set -e

API_KEY="${1:-}"  # Pass API key as first argument if you have it

configure_with_api_key() {
    local SERVICE=$1
    local URL=$2
    local API_KEY=$3
    
    echo "   Configuring $SERVICE..."
    
    curl -s -X PUT "$URL/api/v3/config/host" \
        -H "X-Api-Key: $API_KEY" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin","authentication":"forms","authenticationRequired":"enabled"}' \
        && echo "   ✅ $SERVICE configured!" \
        || echo "   ⚠️  Failed to configure $SERVICE"
}

echo "🔧 Configuring *arr applications with API keys..."
echo ""
echo "⚠️  This requires API keys from each service."
echo "   Get them from: Settings → General → API Key"
echo ""
echo "Usage: $0 <sonarr_key> <radarr_key> <prowlarr_key>"
echo ""

if [ -z "$API_KEY" ]; then
    echo "   Skipping API-based configuration"
    echo "   Services will be accessible without auth initially"
    echo "   Configure manually or provide API keys"
    exit 0
fi

# This is a placeholder - you'd need to pass API keys for each service
# configure_with_api_key "Sonarr" "http://localhost:8989" "$SONARR_KEY"
# configure_with_api_key "Radarr" "http://localhost:7878" "$RADARR_KEY"  
# configure_with_api_key "Prowlarr" "http://localhost:9696" "$PROWLARR_KEY"

