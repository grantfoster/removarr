# Docker/Podman Build Fixes

## Issues Fixed

1. **Build Error**: `stat /app/cmd/removarr: directory not found`
   - **Fix**: Added explicit COPY commands for all required directories (cmd/, internal/, web/, migrations/)
   - The `web/` directory was missing, which contains templates and static files

2. **Port Binding Error**: `Failed to bind port 8080`
   - **Fix**: Updated default port to 31111 in docker-compose.yml
   - Updated REMOVARR_PORT default to 31111
   - Updated Dockerfile EXPOSE to 31111
   - Updated port mapping in docker-compose.yml

3. **Config Port Mismatch**: Config was hardcoded to 31111
   - **Fix**: Made config.yaml generation use environment variable: `port: ${REMOVARR_PORT:-31111}`

## Changes Made

### docker-compose.yml
- Changed default `REMOVARR_PORT` from 8080 to 31111
- Updated port mapping to use 31111:31111
- Made config.yaml generation use `REMOVARR_PORT` env var

### Dockerfile
- Added explicit COPY for `web/` directory
- Updated EXPOSE to 31111
- Copied web directory to final image

## To Deploy

```bash
# Set your port (if different from 31111)
export REMOVARR_PORT=31111
export POSTGRES_PASSWORD=your_secure_password
export SESSION_SECRET=your_session_secret

# Build and start
podman-compose up -d --build

# Or if you need to rebuild from scratch
podman-compose down
podman-compose up -d --build
```

## Troubleshooting

If you still get build errors:
1. Make sure you're in the project root directory
2. Check that all directories exist: `ls -la cmd/ internal/ web/ migrations/`
3. Clean up old images: `podman rmi removarr_removarr` (if it exists)
4. Try building manually: `podman build -t removarr_removarr .`

If port binding fails:
1. Check if port 31111 is available: `netstat -tuln | grep 31111`
2. Change port in environment: `export REMOVARR_PORT=31112`
3. Update docker-compose.yml port mapping accordingly

