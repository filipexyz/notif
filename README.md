# notif.sh

**Managed pub/sub for devs who don't want to manage infra.**

notif.sh is an open-source event hub that centralizes events from any source and reliably delivers them to any consumer. Built for AI agents, automations, and integrations.

```bash
# Publish an event
curl -X POST http://localhost:8080/api/v1/emit \
  -H "Authorization: Bearer nsh_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{"topic": "leads.new", "data": {"name": "John", "email": "john@example.com"}}'
```

## Why notif.sh?

**The Problem:** Connecting events between systems is painful.

- **Self-managed queues** (Redis, NATS, Kafka) require infrastructure, retry logic, dead letter handling, and custom APIs
- **Direct webhooks** lose messages when endpoints are offline, have no built-in retry, and each integration is a new endpoint
- **AI agents** need reliable event delivery for task assignments, status updates, and approvals

**The Solution:** A simple event hub with persistence, auto-retry, dead letter queues, and real-time delivery via WebSocket or webhooks.

## Features

- **Topics** — Named channels with wildcard support (`leads.*`, `agent.*.error`)
- **Publish** — Simple HTTP POST from anywhere
- **Subscribe** — Real-time WebSocket with explicit ack/nack
- **Webhooks** — Deliver events to HTTP endpoints with HMAC signing
- **Persistence** — Events stored in NATS JetStream (configurable retention)
- **Auto-retry** — Configurable backoff with max retries
- **Dead Letter Queue** — Events that fail too many times are preserved for replay
- **Consumer Groups** — Load-balance events across multiple instances
- **Unified Delivery Tracking** — See all deliveries (WebSocket + webhooks) for any event
- **Dashboard** — Web UI for monitoring events, webhooks, and DLQ

## Quick Start

### Prerequisites

- Go 1.21+
- Docker and Docker Compose

### 1. Clone and Start

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

The server runs at `http://localhost:8080`.

### 2. Create an API Key

```bash
# Connect to the database and create a key
PGPASSWORD=notif_dev psql -h localhost -U notif -d notif -c "
INSERT INTO api_keys (key_hash, key_prefix, name, org_id)
VALUES (
  encode(sha256('nsh_mydevkey12345678901234abcd'::bytea), 'hex'),
  'nsh_mydevkey',
  'Dev Key',
  'org_dev'
) RETURNING id;"
```

Use `nsh_mydevkey12345678901234abcd` as your API key.

### 3. Publish an Event

```bash
curl -X POST http://localhost:8080/api/v1/emit \
  -H "Authorization: Bearer nsh_mydevkey12345678901234abcd" \
  -H "Content-Type: application/json" \
  -d '{"topic": "orders.new", "data": {"order_id": "12345", "amount": 99.99}}'
```

### 4. Subscribe via WebSocket

```javascript
const ws = new WebSocket('ws://localhost:8080/ws?token=nsh_mydevkey12345678901234abcd')

ws.onopen = () => {
  ws.send(JSON.stringify({
    action: 'subscribe',
    topics: ['orders.*'],
    options: { auto_ack: false }
  }))
}

ws.onmessage = (msg) => {
  const data = JSON.parse(msg.data)
  if (data.type === 'event') {
    console.log('Received:', data.topic, data.data)
    ws.send(JSON.stringify({ action: 'ack', id: data.id }))
  }
}
```

## API Reference

### Authentication

All API requests require an API key via:
- Header: `Authorization: Bearer nsh_xxx`
- Query param (WebSocket): `?token=nsh_xxx`

### Publish Events

```http
POST /api/v1/emit
Content-Type: application/json
Authorization: Bearer nsh_xxx

{
  "topic": "leads.new",
  "data": {"name": "John", "email": "john@example.com"}
}
```

**Response:**

```json
{
  "id": "evt_abc123...",
  "topic": "leads.new",
  "timestamp": "2025-01-15T10:30:00Z"
}
```

### Query Events

```http
GET /api/v1/events?topic=leads.*&limit=100
```

### Get Event Deliveries

```http
GET /api/v1/events/{event_id}/deliveries
```

Returns all delivery attempts (WebSocket and webhooks) for an event.

### Webhooks

```http
# Create webhook
POST /api/v1/webhooks
{
  "url": "https://my-api.com/hook",
  "topics": ["leads.*", "orders.>"]
}

# List webhooks
GET /api/v1/webhooks

# Update webhook
PUT /api/v1/webhooks/{id}

# Delete webhook
DELETE /api/v1/webhooks/{id}
```

Webhook requests include HMAC-SHA256 signature in `X-Notif-Signature` header.

### Dead Letter Queue

```http
# List DLQ messages
GET /api/v1/dlq?topic=leads.*

# Replay a message
POST /api/v1/dlq/{seq}/replay

# Replay all messages
POST /api/v1/dlq/replay-all?topic=leads.*

# Delete a message
DELETE /api/v1/dlq/{seq}

# Purge all
DELETE /api/v1/dlq/purge
```

## WebSocket Protocol

### Connect

```
ws://localhost:8080/ws?token=nsh_xxx
```

### Subscribe

```json
{
  "action": "subscribe",
  "topics": ["leads.*", "orders.>"],
  "options": {
    "auto_ack": false,
    "from": "latest",
    "group": "my-consumer-group",
    "max_retries": 5,
    "ack_timeout": "5m"
  }
}
```

| Option | Description |
|--------|-------------|
| `auto_ack` | Auto-acknowledge on delivery (default: `true`) |
| `from` | Start position: `"latest"`, `"beginning"`, or ISO8601 timestamp |
| `group` | Consumer group name for load balancing |
| `max_retries` | Max retry attempts before DLQ (default: `5`) |
| `ack_timeout` | Time to wait for ack before retry (default: `"5m"`) |

### Receive Events

```json
{
  "type": "event",
  "id": "evt_abc123...",
  "topic": "leads.new",
  "data": {"name": "John"},
  "timestamp": "2025-01-15T10:30:00Z",
  "attempt": 1,
  "max_attempts": 5
}
```

### Acknowledge

```json
{"action": "ack", "id": "evt_abc123..."}
```

### Negative Acknowledge (retry)

```json
{"action": "nack", "id": "evt_abc123...", "retry_in": "5m"}
```

## Topic Patterns

| Pattern | Matches |
|---------|---------|
| `leads.new` | Exactly `leads.new` |
| `leads.*` | `leads.new`, `leads.qualified` (single segment) |
| `orders.>` | `orders.new`, `orders.processing.paid` (all remaining) |
| `agent.*.error` | `agent.search.error`, `agent.research.error` |

## Configuration

Environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | required |
| `NATS_URL` | NATS server URL | required |
| `CLERK_SECRET_KEY` | Clerk auth (for dashboard) | optional |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `LOG_FORMAT` | Log format (json, text) | `json` |
| `SHUTDOWN_TIMEOUT` | Graceful shutdown timeout | `30s` |

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

**Tech Stack:**
- **Backend:** Go, Chi router
- **Messaging:** NATS JetStream
- **Database:** PostgreSQL (sqlc for type-safe queries)
- **Frontend:** TanStack Start (React 19 + TanStack Router)
- **Auth:** Clerk (dashboard) + API keys (programmatic access)

## Development

```bash
# Start dependencies
make dev

# Run migrations
make migrate

# Start server with hot reload
make watch

# Run tests
make test

# Run e2e tests (requires Docker)
make test-e2e

# Generate sqlc code
make generate

# Build binaries
make build
```

### Frontend Development

```bash
cd web
npm install
npm run dev   # Runs on :3000
```

## Project Structure

```
notif/
├── cmd/notifd/          # Server entry point
├── internal/
│   ├── handler/         # HTTP handlers
│   ├── middleware/      # Auth, logging
│   ├── nats/            # JetStream integration
│   ├── websocket/       # WebSocket client/hub
│   ├── webhook/         # Webhook delivery worker
│   └── db/              # Database queries (sqlc)
├── db/
│   ├── migrations/      # Goose migrations
│   └── queries/         # SQL queries
├── web/                 # Frontend (TanStack Start)
└── tests/e2e/           # Integration tests
```

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting a pull request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
