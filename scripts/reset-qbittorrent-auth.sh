#!/bin/bash

# Reset qBittorrent authentication to get a fresh temporary password
# This removes the password hash so qBittorrent generates a new temp password

set -e

CONTAINER="removarr-test-qbittorrent"
CONFIG_PATH="/config/qBittorrent/qBittorrent.conf"

echo "🔧 Resetting qBittorrent authentication..."

# Stop qBittorrent
echo "   Stopping qBittorrent..."
docker stop "$CONTAINER" || true

sleep 2

# Backup current config
echo "   Backing up current config..."
docker exec "$CONTAINER" sh -c "cp $CONFIG_PATH ${CONFIG_PATH}.bak" 2>/dev/null || true

# Read current config
docker exec "$CONTAINER" cat "$CONFIG_PATH" > /tmp/qb_config_backup.conf 2>/dev/null || {
    echo "   Creating new config..."
    docker exec "$CONTAINER" sh -c "mkdir -p /config/qBittorrent" || true
}

# Remove password hash from config (this will make qBittorrent generate a new temp password)
echo "   Removing password hash..."
docker exec "$CONTAINER" sh -c "sed -i '/^WebUI\\\\Password_PBKDF2/d' $CONFIG_PATH" 2>/dev/null || \
docker exec "$CONTAINER" sh -c "sed -i '' '/^WebUI\\\\Password_PBKDF2/d' $CONFIG_PATH" 2>/dev/null || true

# Ensure WebUI settings are correct (but without password)
docker exec "$CONTAINER" sh -c "cat $CONFIG_PATH" > /tmp/qb_config_current.conf 2>/dev/null || touch /tmp/qb_config_current.conf

# Remove old WebUI settings
sed -i.bak '/^WebUI\\/d' /tmp/qb_config_current.conf 2>/dev/null || sed -i '' '/^WebUI\\/d' /tmp/qb_config_current.conf 2>/dev/null || true

# Add WebUI settings without password
if ! grep -q "^\[Preferences\]" /tmp/qb_config_current.conf; then
    echo "" >> /tmp/qb_config_current.conf
    echo "[Preferences]" >> /tmp/qb_config_current.conf
fi

cat >> /tmp/qb_config_current.conf << 'EOF'
WebUI\Enabled=true
WebUI\Address=0.0.0.0
WebUI\Port=8080
WebUI\LocalHostAuth=false
WebUI\Username=admin
WebUI\CSRFProtection=false
WebUI\ClickjackingProtection=false
WebUI\HostHeaderValidation=false
WebUI\ServerDomains=*
EOF

# Copy config back
docker cp /tmp/qb_config_current.conf "$CONTAINER:$CONFIG_PATH"

# Remove lockfile
docker exec "$CONTAINER" rm -f /config/qBittorrent/lockfile 2>/dev/null || true

# Start qBittorrent
echo "   Starting qBittorrent..."
docker start "$CONTAINER"

echo ""
echo "   Waiting for qBittorrent to start and generate temp password..."
sleep 8

# Get the temporary password
TEMP_PASS=$(docker logs "$CONTAINER" 2>&1 | grep -i "temporary password" | tail -1 | sed -n 's/.*: \([^ ]*\)/\1/p' || echo "")

if [ -n "$TEMP_PASS" ]; then
    echo ""
    echo "✅ qBittorrent reset!"
    echo ""
    echo "   Access: http://localhost:8081"
    echo "   Username: admin"
    echo "   Temporary Password: $TEMP_PASS"
    echo ""
    echo "   After logging in, go to:"
    echo "   Tools → Options → Web UI"
    echo "   Set password to: admin"
    echo "   Save and logout/login"
else
    echo ""
    echo "✅ qBittorrent reset (but couldn't find temp password in logs)"
    echo ""
    echo "   Check logs: docker logs $CONTAINER | grep -i password"
    echo "   Or access http://localhost:8081 and try:"
    echo "   Username: admin"
    echo "   Password: (check logs for temp password)"
fi

