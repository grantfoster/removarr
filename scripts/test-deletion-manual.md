# Manual Testing Guide for Deletion Flow

Since getting real media files is difficult, here's a step-by-step guide to test the deletion flow using minimal setup.

## Prerequisites

1. All services running (Overseerr, Radarr, Sonarr, qBittorrent)
2. Removarr running and connected to all services
3. Admin user created in Removarr

## Option 1: Test with TMDB Metadata Only (No Real Files)

### Step 1: Add a Movie to Radarr (No Download)

1. Open Radarr UI
2. Go to "Add New" → "Add Movie"
3. Search for "The Dark Knight" (TMDB ID: 49026)
4. Click "Add Movie"
5. **Important**: Set "Root Folder" to `/movies` (or your test path)
6. **DO NOT** trigger a search/download - just add it monitored
7. Note the Radarr ID (shown in URL or movie details)

### Step 2: Create a Fake Request in Overseerr (Optional)

If you want to test Overseerr integration:

1. Open Overseerr UI
2. Search for "The Dark Knight"
3. Click "Request" (but don't actually approve/download)
4. Note the Request ID

### Step 3: Sync in Removarr

1. Open Removarr dashboard
2. Click "Sync Media"
3. Wait for sync to complete
4. Verify "The Dark Knight" appears in the list
5. It should show as "Not Downloaded" (since we didn't actually download)

### Step 4: Test Deletion

1. Click "Delete" on "The Dark Knight"
2. Confirm in the modal
3. Check logs: `tail -f /tmp/removarr.log`
4. Verify:
   - Movie was unmonitored/deleted from Radarr
   - Request was deleted from Overseerr (if you created one)
   - Entry removed from Removarr dashboard

## Option 2: Test with Dummy Torrent (No Real Files)

### Step 1: Add Movie to Radarr (Same as Option 1)

### Step 2: Create a Dummy Torrent in qBittorrent

1. Create a small dummy file:
   ```bash
   mkdir -p /tmp/test-movie
   echo "This is a test file" > /tmp/test-movie/test-video.mkv
   ```

2. Create a dummy torrent file (or use a test torrent):
   ```bash
   # Use a test torrent site or create a minimal .torrent file
   # For testing, we can just add a placeholder
   ```

3. Add to qBittorrent:
   - Open qBittorrent UI
   - Add torrent (choose the dummy file)
   - Set download location to `/movies/The Dark Knight (2012)`
   - Let it "download" (it will just create the folder structure)

### Step 3: Link Torrent to Media in Removarr

You may need to manually link the torrent hash to the media item in the database, or wait for the torrent sync to match based on folder paths.

### Step 4: Test Deletion

Same as Option 1, Step 4, but now it will also:
- Delete the dummy files from `/tmp/test-movie`
- Remove the torrent from qBittorrent

## Option 3: Unit Test Individual Components

Test each integration separately:

### Test Radarr Deletion Only

```bash
# In Radarr, add a test movie (monitored)
# Then use curl to test deletion:
curl -X DELETE "http://localhost:7878/api/v3/movie/1" \
  -H "X-Api-Key: YOUR_RADARR_API_KEY"
```

### Test Overseerr Deletion Only

```bash
# Create a request in Overseerr
# Then delete it:
curl -X DELETE "http://localhost:5055/api/v1/request/1" \
  -H "X-Api-Key: YOUR_OVERSEERR_API_KEY"
```

### Test qBittorrent Deletion Only

```bash
# Add a test torrent
# Get its hash
# Then delete it:
curl -X GET "http://localhost:8081/api/v2/torrents/delete?hashes=TORRENT_HASH&deleteFiles=true" \
  -b /tmp/qb-cookies.txt
```

## Verification Checklist

After running deletion, verify:

- [ ] Media item removed from Removarr dashboard
- [ ] Movie unmonitored or deleted in Radarr
- [ ] Request deleted in Overseerr (if applicable)
- [ ] Torrent removed from qBittorrent (if applicable)
- [ ] Files deleted from filesystem (if downloaded)
- [ ] Audit log entry created in Removarr database
- [ ] No errors in `/tmp/removarr.log`

## Database Verification

Check the audit log:

```sql
psql -h localhost -p 5433 -U removarr -d removarr -c \
  "SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT 5;"
```

Check if media item was deleted:

```sql
psql -h localhost -p 5433 -U removarr -d removarr -c \
  "SELECT COUNT(*) FROM media_items WHERE title LIKE '%Dark Knight%';"
```

## Troubleshooting

### Movie not appearing after sync
- Check Radarr API is accessible
- Verify movie is actually in Radarr
- Check Removarr logs for sync errors

### Deletion not working
- Check all integration APIs are accessible
- Verify API keys are correct
- Check `/tmp/removarr.log` for detailed error messages
- Ensure services are running

### Files not deleted
- Check file permissions
- Verify file path is correct in database
- Check if files actually exist (they might not for test movies)

