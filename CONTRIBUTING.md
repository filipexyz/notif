# Contributing to notif.sh

This guide covers self-hosting and development setup for notif.sh.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         notif.sh                            │
│                                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ REST API │  │WebSocket │  │ Webhooks │  │Dashboard │   │
│  │  /emit   │  │   /ws    │  │  Worker  │  │  (React) │   │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────────┘   │
│       │             │             │                        │
│       └─────────────┼─────────────┘                        │
│                     ▼                                       │
│              ┌─────────────┐      ┌─────────────┐          │
│              │    NATS     │      │  PostgreSQL │          │
│              │ (JetStream) │      │ (metadata)  │          │
│              └─────────────┘      └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

## Tech Stack

- **Backend**: Go, Chi router
- **Messaging**: NATS JetStream
- **Database**: PostgreSQL (sqlc)
- **Frontend**: TanStack Start (React 19)
- **Auth**: Clerk (dashboard) + API keys

## Prerequisites

- Go 1.21+
- Docker and Docker Compose
- Node.js 18+ (for frontend/SDKs)
- Python 3.11+ (for Python SDK)

## Development Setup

### 1. Clone and Start Services

```bash
git clone https://github.com/filipexyz/notif.git
cd notif

# Start NATS and PostgreSQL
make dev

# Run migrations
make migrate

# Start the server
make run
```

Server runs at `http://localhost:8080`.

### 2. Create a Dev API Key

```bash
make seed
```

This creates a test key: `nsh_test_abcdefghij12345678901234`

Or manually:

```bash
PGPASSWORD=notif_dev psql -h localhost -U notif -d notif -c "
INSERT INTO api_keys (key_hash, key_prefix, name, org_id)
VALUES (
  encode(sha256('nsh_mydevkey12345678901234abcd'::bytea), 'hex'),
  'nsh_mydevkey',
  'Dev Key',
  'org_dev'
) RETURNING id;"
```

### 3. Test the API

```bash
curl -X POST http://localhost:8080/api/v1/emit \
  -H "Authorization: Bearer nsh_test_abcdefghij12345678901234" \
  -H "Content-Type: application/json" \
  -d '{"topic": "test", "data": {"hello": "world"}}'
```

## Make Commands

```bash
make dev          # Start NATS + Postgres (Docker)
make run          # Run server
make watch        # Run with hot reload (requires air)
make build        # Build server binary
make build-cli    # Build CLI binary
make test         # Run all tests (Go + TypeScript + Python)
make test-e2e     # Run e2e tests
make migrate      # Run database migrations
make generate     # Generate sqlc code
make seed         # Create test API key
```

## Frontend Development

```bash
cd web
npm install
npm run dev   # Runs on :3000
```

Requires `CLERK_SECRET_KEY` for authentication.

## Project Structure

```
notif/
├── cmd/
│   ├── notifd/           # Server entrypoint
│   └── notif/            # CLI entrypoint
├── internal/
│   ├── server/           # HTTP server, routes
│   ├── handler/          # Request handlers
│   ├── middleware/       # Auth (unified, clerk)
│   ├── nats/             # NATS JetStream
│   ├── websocket/        # WebSocket hub
│   ├── webhook/          # Webhook delivery
│   ├── db/               # sqlc generated code
│   └── domain/           # Business logic
├── db/
│   ├── migrations/       # Goose migrations
│   └── queries/          # sqlc queries
├── pkg/client/           # Go SDK
├── sdk/
│   ├── typescript/       # TypeScript SDK (npm: notif.sh)
│   └── python/           # Python SDK (pip: notifsh)
├── web/                  # Frontend (TanStack Start)
└── tests/e2e/            # E2E tests (testcontainers)
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `DATABASE_URL` | PostgreSQL connection | required |
| `NATS_URL` | NATS server URL | required |
| `CLERK_SECRET_KEY` | Clerk auth (dashboard) | optional |
| `LOG_LEVEL` | debug, info, warn, error | `info` |
| `LOG_FORMAT` | json, text | `json` |

## API Endpoints

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/health` | Liveness check |
| GET | `/ready` | Readiness check |
| GET | `/ws` | WebSocket subscription |
| POST | `/api/v1/emit` | Publish event |
| GET | `/api/v1/events` | List events |
| POST | `/api/v1/webhooks` | Create webhook |
| GET | `/api/v1/webhooks` | List webhooks |
| GET | `/api/v1/dlq` | List dead letter queue |
| POST | `/api/v1/dlq/:seq/replay` | Replay DLQ message |

## WebSocket Protocol

### Subscribe

```json
{
  "action": "subscribe",
  "topics": ["orders.*"],
  "options": {
    "auto_ack": false,
    "from": "latest",
    "group": "worker-1"
  }
}
```

### Acknowledge

```json
{"action": "ack", "id": "evt_xxx"}
{"action": "nack", "id": "evt_xxx", "retry_in": "5m"}
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Run tests (`make test`)
4. Commit your changes
5. Push to the branch
6. Open a Pull Request

## Publishing SDKs

```bash
make publish-ts    # Publish TypeScript SDK to npm
make publish-py    # Publish Python SDK to PyPI
make publish-sdks  # Publish both
```
