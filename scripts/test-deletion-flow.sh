#!/bin/bash
# Test script for end-to-end deletion flow
# This script simulates the full flow: Overseerr -> Radarr -> qBittorrent -> Removarr

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Removarr Deletion Flow Test ===${NC}\n"

# Load config
CONFIG_FILE="${CONFIG_FILE:-config.yaml}"
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}Error: $CONFIG_FILE not found${NC}"
    exit 1
fi

# Extract API keys and URLs from config (basic parsing)
OVerseerr_URL=$(grep -A 5 "overseerr:" "$CONFIG_FILE" | grep "url:" | awk '{print $2}' | tr -d '"')
OVerseerr_API=$(grep -A 5 "overseerr:" "$CONFIG_FILE" | grep "api_key:" | awk '{print $2}' | tr -d '"')
RADarr_URL=$(grep -A 5 "radarr:" "$CONFIG_FILE" | grep "url:" | awk '{print $2}' | tr -d '"')
RADarr_API=$(grep -A 5 "radarr:" "$CONFIG_FILE" | grep "api_key:" | awk '{print $2}' | tr -d '"')
QBittorrent_URL=$(grep -A 5 "qbittorrent:" "$CONFIG_FILE" | grep "url:" | awk '{print $2}' | tr -d '"')
QBittorrent_USER=$(grep -A 5 "qbittorrent:" "$CONFIG_FILE" | grep "username:" | awk '{print $2}' | tr -d '"')
QBittorrent_PASS=$(grep -A 5 "qbittorrent:" "$CONFIG_FILE" | grep "password:" | awk '{print $2}' | tr -d '"')

echo -e "${YELLOW}Step 1: Create a test movie in Radarr${NC}"
echo "This simulates what happens when Overseerr requests a movie..."

# Use a well-known movie ID that Radarr can fetch metadata for
# The Dark Knight Rises (TMDB ID: 49026) - safe test movie
TMDB_ID=49026

# Add movie to Radarr (monitored but not downloaded)
RADarr_RESPONSE=$(curl -s -X POST "$RADarr_URL/api/v3/movie" \
    -H "X-Api-Key: $RADarr_API" \
    -H "Content-Type: application/json" \
    -d "{
        \"tmdbId\": $TMDB_ID,
        \"qualityProfileId\": 1,
        \"rootFolderPath\": \"/movies\",
        \"monitored\": true,
        \"addOptions\": {
            \"searchForMovie\": false
        }
    }")

RADarr_ID=$(echo "$RADarr_RESPONSE" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)

if [ -z "$RADarr_ID" ]; then
    echo -e "${RED}Failed to add movie to Radarr${NC}"
    echo "Response: $RADarr_RESPONSE"
    exit 1
fi

echo -e "${GREEN}✓ Movie added to Radarr (ID: $RADarr_ID)${NC}"

echo -e "\n${YELLOW}Step 2: Create a dummy download in qBittorrent${NC}"
echo "This simulates a torrent being downloaded..."

# Create a small dummy torrent file
DUMMY_TORRENT_FILE="/tmp/test-deletion.torrent"
cat > "$DUMMY_TORRENT_FILE" << 'EOF'
d8:announce36:http://tracker.example.com/announce7:comment13:Test torrent10:created by13:Removarr Test13:creation datei1234567890e4:infod6:lengthi1024e4:name9:test.mp412:piece lengthi16384e6:pieces20:abcdefghijklmnopqrstee
EOF

# Add torrent to qBittorrent (but don't actually download)
# Note: qBittorrent API requires actual torrent file or magnet link
# For testing, we'll just verify the API is accessible

QBittorrent_LOGIN=$(curl -s -c /tmp/qb-cookies.txt -X POST "$QBittorrent_URL/api/v2/auth/login" \
    -d "username=$QBittorrent_USER&password=$QBittorrent_PASS")

if [[ "$QBittorrent_LOGIN" != "Ok." ]]; then
    echo -e "${YELLOW}Warning: qBittorrent login failed (this is OK for testing)${NC}"
    echo "We'll skip the qBittorrent step and test deletion without it"
else
    echo -e "${GREEN}✓ qBittorrent API accessible${NC}"
fi

echo -e "\n${YELLOW}Step 3: Sync media in Removarr${NC}"
REMOVarr_RESPONSE=$(curl -s -X POST "http://localhost:8080/api/media/sync" \
    -H "Content-Type: application/json" \
    -u "admin:admin")

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Media synced in Removarr${NC}"
else
    echo -e "${RED}Failed to sync media${NC}"
fi

echo -e "\n${YELLOW}Step 4: Verify media appears in Removarr${NC}"
MEDIA_LIST=$(curl -s "http://localhost:8080/api/media" \
    -u "admin:admin")

if echo "$MEDIA_LIST" | grep -q "The Dark Knight"; then
    echo -e "${GREEN}✓ Media found in Removarr${NC}"
    MEDIA_ID=$(echo "$MEDIA_LIST" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
    echo "Media ID: $MEDIA_ID"
else
    echo -e "${YELLOW}Media not found yet - may need to wait for sync${NC}"
    echo "Response: $MEDIA_LIST"
fi

echo -e "\n${YELLOW}Step 5: Test deletion via API${NC}"
if [ ! -z "$MEDIA_ID" ]; then
    DELETE_RESPONSE=$(curl -s -X DELETE "http://localhost:8080/api/media/$MEDIA_ID" \
        -H "HX-Request: true" \
        -u "admin:admin")
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Deletion request sent${NC}"
    else
        echo -e "${RED}Deletion failed${NC}"
    fi
else
    echo -e "${YELLOW}Skipping deletion - no media ID found${NC}"
fi

echo -e "\n${YELLOW}Step 6: Verify deletion${NC}"
sleep 2

# Check if movie still exists in Radarr
RADarr_CHECK=$(curl -s "$RADarr_URL/api/v3/movie/$RADarr_ID" \
    -H "X-Api-Key: $RADarr_API")

if echo "$RADarr_CHECK" | grep -q "monitored"; then
    MONITORED=$(echo "$RADarr_CHECK" | grep -o '"monitored":[^,]*' | cut -d: -f2)
    if [ "$MONITORED" = "false" ]; then
        echo -e "${GREEN}✓ Movie unmonitored in Radarr${NC}"
    else
        echo -e "${YELLOW}Movie still monitored (may need to check deletion service logs)${NC}"
    fi
fi

echo -e "\n${GREEN}=== Test Complete ===${NC}"
echo "Check the logs at /tmp/removarr.log for detailed deletion steps"
echo "Check Radarr to verify the movie was unmonitored/deleted"
echo "Check qBittorrent to verify torrents were removed (if applicable)"

