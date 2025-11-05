#!/bin/bash

# Fix qBittorrent authentication by using the temp password to configure it
# This is the most reliable method since qBittorrent generates random passwords

set -e

CONTAINER="removarr-test-qbittorrent"
QB_URL="http://localhost:8081"
QB_USER="admin"

echo "🔧 Fixing qBittorrent authentication..."

# Check if container is running
if ! docker ps | grep -q "$CONTAINER"; then
    echo "❌ Container $CONTAINER is not running"
    echo "   Start it first: docker-compose -f docker-compose.test.yml up -d qbittorrent"
    exit 1
fi

# Get the temporary password from logs
echo "   Getting temporary password from logs..."
TEMP_PASS=$(docker logs "$CONTAINER" 2>&1 | grep -i "temporary password" | tail -1 | sed -n 's/.*: \([^ ]*\)/\1/p')

if [ -z "$TEMP_PASS" ]; then
    echo "   ⚠️  Could not find temporary password in logs"
    echo "   Trying to use default adminadmin..."
    TEMP_PASS="adminadmin"
fi

echo "   Found password: $TEMP_PASS"

# Wait for qBittorrent to be ready
echo "   Waiting for qBittorrent to be ready..."
max_attempts=30
attempt=0
while [ $attempt -lt $max_attempts ]; do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$QB_URL/api/v2/app/version" 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "403" ]; then
        break
    fi
    attempt=$((attempt + 1))
    sleep 2
done

if [ $attempt -eq $max_attempts ]; then
    echo "   ❌ qBittorrent not responding"
    exit 1
fi

# Login with temp password
echo "   Logging in with temporary password..."
RESPONSE=$(curl -s -c /tmp/qb_cookies.txt -X POST "$QB_URL/api/v2/auth/login" \
    -d "username=$QB_USER&password=$TEMP_PASS" 2>/dev/null || echo "Failed")

if [ "$RESPONSE" != "Ok." ]; then
    echo "   ⚠️  Could not login. Trying alternative methods..."
    echo ""
    echo "   Manual steps:"
    echo "   1. Access $QB_URL"
    echo "   2. The password should be: $TEMP_PASS"
    echo "   3. If you see a login form, use:"
    echo "      Username: admin"
    echo "      Password: $TEMP_PASS"
    echo "   4. Once logged in, go to Tools → Options → Web UI"
    echo "   5. Set username: admin, password: adminadmin"
    echo "   6. Save and logout/login"
    exit 0
fi

echo "   ✅ Logged in successfully!"

# Set password to adminadmin
echo "   Setting password to adminadmin..."
RESPONSE=$(curl -s -b /tmp/qb_cookies.txt -X POST "$QB_URL/api/v2/app/setPreferences" \
    -d "json={\"web_ui_password\":\"adminadmin\"}" 2>/dev/null || echo "Failed")

if [ "$RESPONSE" = "Ok." ]; then
    echo "   ✅ Password set to adminadmin"
else
    echo "   ⚠️  Could not set password via API"
    echo "   You may need to set it manually in the web UI"
fi

# Logout
curl -s -b /tmp/qb_cookies.txt -X POST "$QB_URL/api/v2/auth/logout" > /dev/null 2>&1

echo ""
echo "✅ qBittorrent configured!"
echo ""
echo "   Access: $QB_URL"
echo "   Username: admin"
echo "   Password: adminadmin"
echo ""
echo "   If you still see 'Unauthorized' with no login form:"
echo "   1. Clear browser cache/cookies for localhost:8081"
echo "   2. Try incognito/private mode"
echo "   3. Wait 10-15 seconds for qBittorrent to fully restart"
echo "   4. Try accessing: $QB_URL/login"

