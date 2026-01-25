# Self-Hosting notif.sh

Run your own notif.sh instance in minutes.

## Quick Start

### 1. Start the Stack

```bash
# Clone the repo
git clone https://github.com/filipexyz/notif.git
cd notif

# Start everything
docker compose -f docker-compose.selfhost.yaml up -d
```

### 2. Bootstrap Your Instance

```bash
curl -X POST http://localhost:8080/api/v1/bootstrap
```

This returns your API key:
```json
{
  "api_key": "nsh_abc123...",
  "project_id": "prj_xxx",
  "message": "Instance bootstrapped successfully. Save this API key - it won't be shown again!"
}
```

**⚠️ Save this key!** It cannot be retrieved later.

### 3. Test It

```bash
# Set your key
export NOTIF_API_KEY=nsh_abc123...

# Emit an event
curl -X POST http://localhost:8080/api/v1/emit \
  -H "Authorization: Bearer $NOTIF_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"topic": "test", "data": {"hello": "world"}}'
```

## Using the CLI

```bash
# Install the CLI
curl -fsSL https://notif.sh/install.sh | sh

# Configure for self-hosted
notif config set server http://localhost:8080
notif auth $NOTIF_API_KEY

# Emit events
notif emit test.hello '{"message": "it works!"}'

# Subscribe to events
notif subscribe 'test.*'
```

## Configuration

Environment variables for the server:

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTH_MODE` | `clerk` | Set to `local` for self-hosted (API keys only) |
| `DEFAULT_ORG_ID` | `org_default` | Org ID for all resources |
| `DATABASE_URL` | required | PostgreSQL connection string |
| `NATS_URL` | `nats://localhost:4222` | NATS server URL |
| `PORT` | `8080` | HTTP server port |
| `LOG_LEVEL` | `info` | debug, info, warn, error |
| `CORS_ORIGINS` | `*` | Allowed CORS origins |

## Architecture

```
┌─────────────────────────────────────────┐
│            Your Application             │
│     (emit events, subscribe, etc)       │
└──────────────────┬──────────────────────┘
                   │ HTTP/WebSocket
                   ▼
┌─────────────────────────────────────────┐
│              notif server               │
│            (port 8080)                  │
└───────┬─────────────────────┬───────────┘
        │                     │
        ▼                     ▼
┌───────────────┐     ┌───────────────┐
│     NATS      │     │   PostgreSQL  │
│  (JetStream)  │     │   (metadata)  │
│  port 4222    │     │   port 5432   │
└───────────────┘     └───────────────┘
```

## Production Considerations

### Security

1. **Change default passwords** in docker-compose
2. **Use HTTPS** - put a reverse proxy (nginx, caddy) in front
3. **Restrict CORS** - set `CORS_ORIGINS` to your domains
4. **Backup PostgreSQL** regularly

### High Availability

For production deployments:

1. Use managed PostgreSQL (RDS, Cloud SQL, etc.)
2. Use NATS cluster for HA
3. Run multiple notif server instances behind a load balancer

### Custom Docker Compose

```yaml
# docker-compose.prod.yaml
services:
  notif:
    image: ghcr.io/filipexyz/notif:latest
    environment:
      AUTH_MODE: "local"
      DATABASE_URL: "postgres://user:pass@your-db:5432/notif?sslmode=require"
      NATS_URL: "nats://your-nats:4222"
      CORS_ORIGINS: "https://app.yourdomain.com"
    restart: always
```

## Updating

```bash
# Pull latest images
docker compose -f docker-compose.selfhost.yaml pull

# Restart with new version
docker compose -f docker-compose.selfhost.yaml up -d
```

Migrations run automatically on startup.

## Troubleshooting

### Check Status

```bash
# Health check
curl http://localhost:8080/health

# Bootstrap status
curl http://localhost:8080/api/v1/bootstrap/status
```

### View Logs

```bash
docker compose -f docker-compose.selfhost.yaml logs -f notif
```

### Reset Everything

```bash
# Stop and remove volumes (WARNING: deletes all data)
docker compose -f docker-compose.selfhost.yaml down -v

# Start fresh
docker compose -f docker-compose.selfhost.yaml up -d
```

## API Keys Management

In self-hosted mode, you manage API keys via the API:

```bash
# List keys
curl -H "Authorization: Bearer $NOTIF_API_KEY" \
  http://localhost:8080/api/v1/api-keys

# Create new key
curl -X POST -H "Authorization: Bearer $NOTIF_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "Production Key"}' \
  http://localhost:8080/api/v1/api-keys

# Revoke key
curl -X DELETE -H "Authorization: Bearer $NOTIF_API_KEY" \
  http://localhost:8080/api/v1/api-keys/{id}
```

## Projects

Organize events by project:

```bash
# Create project
curl -X POST -H "Authorization: Bearer $NOTIF_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "Production"}' \
  http://localhost:8080/api/v1/projects

# List projects
curl -H "Authorization: Bearer $NOTIF_API_KEY" \
  http://localhost:8080/api/v1/projects
```

## Support

- [Documentation](https://docs.notif.sh)
- [GitHub Issues](https://github.com/filipexyz/notif/issues)
- [Discord](https://discord.gg/notif)
