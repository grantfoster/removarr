#!/bin/bash

# Fix qBittorrent Web UI by modifying config to allow external access
# This disables LocalHostAuth which blocks the login form

set -e

CONTAINER="removarr-test-qbittorrent"
CONFIG_PATH="/config/qBittorrent/qBittorrent.conf"

echo "🔧 Fixing qBittorrent Web UI configuration..."

# Check if container is running
if ! docker ps | grep -q "$CONTAINER"; then
    echo "❌ Container $CONTAINER is not running"
    exit 1
fi

# Read current config first (while container is running)
echo "   Reading current configuration..."
docker exec "$CONTAINER" cat "$CONFIG_PATH" > /tmp/qb_config_current.conf 2>/dev/null || {
    echo "   ⚠️  Config file not found, will create new one"
    touch /tmp/qb_config_current.conf
}

# Stop qBittorrent to modify config
echo "   Stopping qBittorrent..."
docker stop "$CONTAINER" || true

# Wait a moment
sleep 2

# Modify the config to disable LocalHostAuth and ensure WebUI is accessible
echo "   Modifying configuration..."

# Copy existing config and modify WebUI settings
cp /tmp/qb_config_current.conf /tmp/qb_config_fixed.conf

# Remove old WebUI settings
sed -i.bak '/^WebUI\\/d' /tmp/qb_config_fixed.conf 2>/dev/null || sed -i '' '/^WebUI\\/d' /tmp/qb_config_fixed.conf 2>/dev/null || true

# Add WebUI settings to Preferences section (or create it)
if ! grep -q "^\[Preferences\]" /tmp/qb_config_fixed.conf; then
    echo "" >> /tmp/qb_config_fixed.conf
    echo "[Preferences]" >> /tmp/qb_config_fixed.conf
fi

# Append WebUI settings
cat >> /tmp/qb_config_fixed.conf << 'EOF'
WebUI\Enabled=true
WebUI\Address=0.0.0.0
WebUI\Port=8080
WebUI\LocalHostAuth=false
WebUI\Username=admin
WebUI\Password_PBKDF2="@ByteArray(AQAAAAEAACcQAAAAEA==)"
WebUI\CSRFProtection=false
WebUI\ClickjackingProtection=false
WebUI\HostHeaderValidation=false
WebUI\ServerDomains=*
EOF

# Copy the fixed config back
docker cp /tmp/qb_config_fixed.conf "$CONTAINER:$CONFIG_PATH"

# Remove lockfile
docker exec "$CONTAINER" rm -f /config/qBittorrent/lockfile 2>/dev/null || true

# Start qBittorrent
echo "   Starting qBittorrent..."
docker start "$CONTAINER"

echo ""
echo "   Waiting for qBittorrent to start..."
sleep 5

echo ""
echo "✅ qBittorrent configuration updated!"
echo ""
echo "   Access: http://localhost:8081"
echo "   Username: admin"
echo "   Password: adminadmin"
echo ""
echo "   The Web UI should now show a login form."
echo "   If you still see 'Unauthorized':"
echo "   1. Wait 10-15 seconds for qBittorrent to fully start"
echo "   2. Hard refresh your browser (Cmd+Shift+R)"
echo "   3. Clear cookies for localhost:8081"
echo "   4. Try incognito/private mode"

