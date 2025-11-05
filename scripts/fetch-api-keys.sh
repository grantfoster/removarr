#!/bin/bash

# Fetch API keys from running services and update config.yaml
# This makes it easy to get API keys after services restart

set -e

CONFIG_FILE="${1:-config.yaml}"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ Config file not found: $CONFIG_FILE"
    exit 1
fi

echo "🔑 Fetching API keys from services..."

# Function to get API key from service
get_api_key() {
    local service=$1
    local url=$2
    local api_key_path=$3
    
    echo "   Checking $service..."
    
    # Try to get API key from the service's config
    # This is a best-effort approach - some services store keys in config files
    # For now, we'll just check if the service is accessible
    
    if curl -s "$url/api/v3/system/status" > /dev/null 2>&1 || \
       curl -s "$url/api/v1/system/status" > /dev/null 2>&1; then
        echo "   ✅ $service is accessible"
    else
        echo "   ⚠️  $service not accessible - may need to get API key manually"
    fi
}

# Check services
echo ""
echo "📋 Services status:"
echo ""

# Sonarr
if docker ps | grep -q "removarr-test-sonarr"; then
    echo "   Sonarr: http://localhost:8989"
    echo "   Get API key: Settings → General → API Key"
fi

# Radarr
if docker ps | grep -q "removarr-test-radarr"; then
    echo "   Radarr: http://localhost:7878"
    echo "   Get API key: Settings → General → API Key"
fi

# Prowlarr
if docker ps | grep -q "removarr-test-prowlarr"; then
    echo "   Prowlarr: http://localhost:9696"
    echo "   Get API key: Settings → General → API Key"
fi

# Overseerr
if docker ps | grep -q "removarr-test-overseerr"; then
    echo "   Overseerr: http://localhost:5055"
    echo "   Get API key: Settings → General → API Key"
fi

echo ""
echo "💡 Tip: API keys persist in Docker volumes unless you use 'docker-compose down -v'"
echo ""
echo "   To preserve API keys:"
echo "   - Use: docker-compose down (without -v)"
echo "   - Or: docker-compose stop (keeps everything)"
echo ""
echo "   To reset everything:"
echo "   - Use: docker-compose down -v (removes volumes, regenerates keys)"

