#!/bin/bash

# Configure qBittorrent to use admin/admin
# qBittorrent defaults to admin/adminadmin, we'll change it to admin/admin

set -e

QB_URL="http://localhost:8081"
QB_USER="admin"
QB_PASS="adminadmin"  # Default password
CONTAINER="removarr-test-qbittorrent"

echo "🔧 Configuring qBittorrent..."

# Wait for qBittorrent to be ready
max_attempts=30
attempt=0

echo "   Waiting for qBittorrent to be ready..."
while [ $attempt -lt $max_attempts ]; do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$QB_URL/api/v2/app/version" 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "403" ]; then
        echo "   ✅ qBittorrent is responding"
        break
    fi
    attempt=$((attempt + 1))
    echo "   ... ($attempt/$max_attempts)"
    sleep 2
done

if [ $attempt -eq $max_attempts ]; then
    echo "   ❌ qBittorrent not ready after $max_attempts attempts"
    echo "   Please configure manually at $QB_URL"
    exit 0
fi

# Try to get the temp password from logs first
echo "   Checking for temporary password in logs..."
TEMP_PASS=$(docker logs "$CONTAINER" 2>&1 | grep -i "temporary password" | tail -1 | sed -n 's/.*: \([^ ]*\)/\1/p' || echo "")

if [ -n "$TEMP_PASS" ]; then
    echo "   Found temporary password in logs, using it..."
    QB_PASS="$TEMP_PASS"
fi

# Try to login with default credentials or temp password
echo "   Logging in with username: $QB_USER, password: $QB_PASS..."
RESPONSE=$(curl -s -c /tmp/qb_cookies.txt -X POST "$QB_URL/api/v2/auth/login" \
    -d "username=$QB_USER&password=$QB_PASS" 2>/dev/null || echo "Failed")

if [ "$RESPONSE" != "Ok." ]; then
    echo "   ⚠️  Could not login with provided credentials."
    echo ""
    echo "   Try the fix script: ./scripts/fix-qbittorrent-auth.sh"
    echo "   Or manually:"
    echo "   1. Check logs for temp password: docker logs $CONTAINER | grep -i password"
    echo "   2. Access $QB_URL"
    echo "   3. Use the temp password from logs"
    echo "   4. Go to Tools → Options → Web UI"
    echo "   5. Set username: admin, password: admin"
    exit 0
fi

echo "   ✅ Logged in successfully"

# Change password to admin
echo "   Setting password to admin..."
RESPONSE=$(curl -s -b /tmp/qb_cookies.txt -X POST "$QB_URL/api/v2/app/setPreferences" \
    -d "json={\"web_ui_password\":\"admin\"}" 2>/dev/null || echo "Failed")

if [ "$RESPONSE" = "Ok." ]; then
    echo "   ✅ Password changed to admin"
else
    echo "   ⚠️  Could not change password via API"
    echo "   You may need to change it manually in the web UI"
fi

# Logout
curl -s -b /tmp/qb_cookies.txt -X POST "$QB_URL/api/v2/auth/logout" > /dev/null 2>&1

echo ""
echo "✅ qBittorrent configured!"
echo "   URL: $QB_URL"
echo "   Username: admin"
echo "   Password: admin"
echo ""
echo "   If you still see 401 errors, try:"
echo "   1. Clear browser cache/cookies"
echo "   2. Access in incognito/private mode"
echo "   3. Or configure manually in the web UI"

