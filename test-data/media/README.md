# Test Media Directory

This directory contains test media files for local testing.

## Structure

```
test-data/media/
├── tv/          # TV shows (for Sonarr)
├── movies/      # Movies (for Radarr)
├── downloads/   # Active downloads (for qBittorrent)
└── complete/     # Completed downloads (for qBittorrent)
```

## Usage

When configuring Sonarr/Radarr:
- **Sonarr Root Folder**: `/tv`
- **Radarr Root Folder**: `/movies`
- **qBittorrent Download Path**: `/downloads`
- **qBittorrent Completed Path**: `/complete`

These paths are mapped from the host `./test-data/media/` directory.

## Creating Test Files

```bash
# Create a test TV episode
mkdir -p test-data/media/tv/TestSeries/Season01
touch test-data/media/tv/TestSeries/Season01/TestSeries.S01E01.mkv
truncate -s 1G test-data/media/tv/TestSeries/Season01/TestSeries.S01E01.mkv

# Create a test movie
mkdir -p test-data/media/movies/TestMovie
touch test-data/media/movies/TestMovie/TestMovie.mkv
truncate -s 2G test-data/media/movies/TestMovie/TestMovie.mkv
```

