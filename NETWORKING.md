# Networking Configuration for Removarr

## Problem

When Removarr runs in a Podman/Docker container, `localhost` or `127.0.0.1` refers to the container itself, not the host machine where Overseerr, Sonarr, Radarr, etc. are running.

## Solutions

### Option 1: Use Hostname/IP (Recommended)

In Removarr Settings, use your seedbox's hostname or IP address instead of `localhost`:

- **Overseerr**: `http://foster-world.box.ca:13914` (or your IP)
- **Sonarr**: `http://foster-world.box.ca:15941` (or your IP)
- **Radarr**: `http://foster-world.box.ca:7878` (or your IP)
- etc.

**To find your IP:**
```bash
hostname -I
# Or
ip addr show | grep "inet " | grep -v 127.0.0.1
```

### Option 2: Use Container Host Aliases

The `docker-compose.yml` includes:
```yaml
extra_hosts:
  - "host.containers.internal:host-gateway"  # Podman
  - "host.docker.internal:host-gateway"       # Docker
```

This allows you to use:
- `http://host.containers.internal:13914` (Podman)
- `http://host.docker.internal:13914` (Docker)

**Note:** This may not work in all Podman setups. Option 1 is more reliable.

### Option 3: Host Network Mode (Not Recommended)

You could use `network_mode: host`, but this:
- Breaks container isolation
- Makes `depends_on` unreliable
- Can cause port conflicts

## Example Settings

Instead of:
```
❌ http://localhost:13914
❌ http://127.0.0.1:13914
```

Use:
```
✅ http://foster-world.box.ca:13914
✅ http://192.168.1.100:13914  (your actual IP)
✅ http://host.containers.internal:13914  (if supported)
```

