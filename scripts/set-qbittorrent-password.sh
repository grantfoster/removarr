#!/bin/bash

# Set qBittorrent password by using the API after first login with temp password
# This script should be run if you see "Invalid Username or Password"

set -e

CONTAINER="removarr-test-qbittorrent"
QB_URL="http://localhost:8081"
QB_USER="admin"
NEW_PASS="admin"

echo "🔧 Setting qBittorrent password..."

# First, try to get the temp password from logs
echo "   Checking for temporary password..."
TEMP_PASS=$(docker logs "$CONTAINER" 2>&1 | grep -i "temporary password" | tail -1 | sed -n 's/.*: \([^ ]*\)/\1/p' || echo "")

if [ -z "$TEMP_PASS" ]; then
    echo "   ⚠️  No temporary password found in logs."
    echo "   This means qBittorrent already has a password set."
    echo ""
    echo "   Options:"
    echo "   1. Reset the config to remove the password:"
    echo "      docker stop $CONTAINER"
    echo "      docker exec $CONTAINER rm -f /config/qBittorrent/qBittorrent.conf"
    echo "      docker start $CONTAINER"
    echo "      Then use the temp password from logs"
    echo ""
    echo "   2. Or try these common passwords:"
    echo "      - adminadmin (default)"
    echo "      - admin"
    echo "      - Check logs: docker logs $CONTAINER | grep -i password"
    exit 1
fi

echo "   Found temporary password: $TEMP_PASS"
echo "   Logging in with temp password..."

# Login with temp password
RESPONSE=$(curl -s -c /tmp/qb_cookies.txt -X POST "$QB_URL/api/v2/auth/login" \
    -d "username=$QB_USER&password=$TEMP_PASS" 2>/dev/null || echo "Failed")

if [ "$RESPONSE" != "Ok." ]; then
    echo "   ❌ Could not login with temporary password"
    echo "   Response: $RESPONSE"
    echo ""
    echo "   Try logging in manually at $QB_URL"
    echo "   Username: admin"
    echo "   Password: $TEMP_PASS"
    exit 1
fi

echo "   ✅ Logged in successfully!"

# Set new password
echo "   Setting password to $NEW_PASS..."
RESPONSE=$(curl -s -b /tmp/qb_cookies.txt -X POST "$QB_URL/api/v2/app/setPreferences" \
    -d "json={\"web_ui_password\":\"$NEW_PASS\"}" 2>/dev/null || echo "Failed")

if [ "$RESPONSE" = "Ok." ]; then
    echo "   ✅ Password set to $NEW_PASS"
else
    echo "   ⚠️  Could not set password via API"
    echo "   Response: $RESPONSE"
    echo "   You may need to set it manually in the web UI"
fi

# Logout
curl -s -b /tmp/qb_cookies.txt -X POST "$QB_URL/api/v2/auth/logout" > /dev/null 2>&1

echo ""
echo "✅ Done!"
echo ""
echo "   Access: $QB_URL"
echo "   Username: admin"
echo "   Password: $NEW_PASS"

