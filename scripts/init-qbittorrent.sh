#!/bin/bash

# Initialize qBittorrent config to disable authentication
# This script should run before qBittorrent starts

set -e

CONFIG_DIR="./test-data/qbittorrent-config"
CONFIG_FILE="$CONFIG_DIR/qBittorrent.conf"

echo "🔧 Creating qBittorrent config..."

mkdir -p "$CONFIG_DIR"

# Create a qBittorrent config file that disables authentication
cat > "$CONFIG_FILE" << 'EOF'
[LegalNotice]
Accepted=true

[Preferences]
Connection\PortRangeMin=6881
WebUI\Enabled=true
WebUI\Address=0.0.0.0
WebUI\Port=8080
WebUI\LocalHostAuth=false
WebUI\Username=admin
WebUI\Password_PBKDF2="@ByteArray(AQAAAAEAACcQAAAAEA==)"
WebUI\CSRFProtection=false
WebUI\ClickjackingProtection=false
EOF

echo "✅ Created qBittorrent config at $CONFIG_FILE"
echo "   This config sets:"
echo "   - Username: admin"
echo "   - Password: adminadmin (default)"
echo "   - LocalHostAuth: false (allows external connections)"
echo ""
echo "   Note: You'll need to mount this config file in docker-compose"

