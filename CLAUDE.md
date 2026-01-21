# notif.sh

Managed pub/sub event hub with webhooks, DLQ, and real-time subscriptions.

## Tech Stack

- **Backend**: Go, Chi router, NATS JetStream
- **Database**: PostgreSQL (sqlc)
- **Auth**: Clerk (JWT) + API keys (`nsh_xxx`)
- **Frontend**: TanStack Start (see `web/CLAUDE.md`)

## Project Structure

```
├── cmd/
│   ├── notifd/         # Server entrypoint
│   └── notif/          # CLI entrypoint
├── internal/
│   ├── server/         # HTTP server, routes
│   ├── handler/        # Request handlers
│   ├── middleware/     # Auth (unified, clerk)
│   ├── nats/           # NATS JetStream (publisher, consumer, DLQ)
│   ├── websocket/      # WebSocket hub
│   ├── scheduler/      # Scheduled events worker
│   ├── codegen/        # Schema codegen (TS/Go from JSON Schema)
│   ├── db/             # sqlc generated code
│   └── domain/         # Business logic
├── db/
│   ├── migrations/     # Goose migrations
│   └── queries/        # sqlc queries
├── pkg/client/         # Go SDK
├── sdk/
│   ├── typescript/     # TypeScript SDK (npm: notif.sh)
│   └── python/         # Python SDK (pip: notifsh)
├── tests/
│   ├── e2e/            # E2E tests (testcontainers)
│   └── integration/    # Integration tests (requires running server)
└── web/                # Frontend (TanStack Start)
```

## API

Base: `http://localhost:8080`

### Auth

All `/api/v1/*` endpoints accept:
- API key: `Authorization: Bearer nsh_xxx`
- Clerk JWT: Session cookie or `Authorization: Bearer <jwt>`

API key format: `nsh_` + 28 alphanumeric chars (regex: `^nsh_[a-zA-Z0-9]{28}$`)

### Endpoints

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/health` | Liveness |
| GET | `/ready` | Readiness |
| GET | `/ws` | WebSocket subscription |
| **Events** | | |
| POST | `/api/v1/emit` | Publish event |
| GET | `/api/v1/events` | List events |
| GET | `/api/v1/events/stats` | Event statistics |
| GET | `/api/v1/events/:seq` | Get event |
| **Webhooks** | | |
| POST | `/api/v1/webhooks` | Create webhook |
| GET | `/api/v1/webhooks` | List webhooks |
| GET | `/api/v1/webhooks/:id` | Get webhook |
| PUT | `/api/v1/webhooks/:id` | Update webhook |
| DELETE | `/api/v1/webhooks/:id` | Delete webhook |
| GET | `/api/v1/webhooks/:id/deliveries` | Deliveries |
| **DLQ** | | |
| GET | `/api/v1/dlq` | List DLQ |
| GET | `/api/v1/dlq/:seq` | Get DLQ message |
| POST | `/api/v1/dlq/:seq/replay` | Replay |
| DELETE | `/api/v1/dlq/:seq` | Delete |
| POST | `/api/v1/dlq/replay-all` | Replay all |
| DELETE | `/api/v1/dlq/purge` | Purge |
| **Stats** | | |
| GET | `/api/v1/stats/overview` | Dashboard stats |
| GET | `/api/v1/stats/events` | Event stats |
| GET | `/api/v1/stats/webhooks` | Webhook stats |
| GET | `/api/v1/stats/dlq` | DLQ stats |
| **Schedules** | | |
| POST | `/api/v1/schedules` | Create scheduled event |
| GET | `/api/v1/schedules` | List scheduled events |
| GET | `/api/v1/schedules/:id` | Get scheduled event |
| DELETE | `/api/v1/schedules/:id` | Cancel scheduled event |
| POST | `/api/v1/schedules/:id/run` | Execute immediately |
| GET | `/api/v1/schedules/stats` | Schedule statistics |
| **API Keys** (Clerk-only) | | |
| POST | `/api/v1/api-keys` | Create key |
| GET | `/api/v1/api-keys` | List keys |
| DELETE | `/api/v1/api-keys/:id` | Revoke key |

### NATS Streams

- `NOTIF_EVENTS`: Events (24h retention, 1GB max)
- `NOTIF_DLQ`: Dead letter queue (7d retention)
- Subjects: `events.<topic>`, `dlq.<topic>`

## SDKs

| SDK | Package | Location |
|-----|---------|----------|
| Go | `github.com/filipexyz/notif/pkg/client` | `pkg/client/` |
| TypeScript | `notif.sh` (npm) | `sdk/typescript/` |
| Python | `notifsh` (pip) | `sdk/python/` |

All SDKs use `NOTIF_API_KEY` env var by default. Core methods: `emit(topic, data)` and `subscribe(...topics)`.

## Development

```bash
make dev          # Start NATS + Postgres (docker)
make watch        # Run server with hot reload (requires air)
make build        # Build server binary
make build-cli    # Build CLI binary
make test         # Run unit tests
make test-e2e     # Run e2e tests (Docker)
make test-integration  # Run integration tests (requires: make dev && make seed && make run)
make migrate      # Run migrations
make generate     # Run sqlc generate
```

## Environment

```
DATABASE_URL=postgres://...
NATS_URL=nats://localhost:4222
CLERK_SECRET_KEY=sk_...
PORT=8080
```

## Anonymous Mode (Frontend)

Bypass Clerk auth for local frontend development. Set in `web/.env`:

```bash
VITE_ANONYMOUS_MODE=true
VITE_DEV_API_KEY=nsh_testkey1234567890abcdefghijk
```

This allows testing the dashboard without signing in. Shows "Anonymous Mode" badge.

## Schema Management

Manage JSON Schemas via CLI with stdin/stdout for piping.

### Create & Edit Schemas

```bash
# Create schema (JSON Schema from stdin)
echo '{"type": "object", "properties": {"id": {"type": "string"}}}' | notif schemas create order-placed --topic "orders.placed"

# Get schema JSON (for piping)
notif schemas get order-placed --schema

# Edit schema (JSON Schema from stdin, auto-bumps version)
cat schema.json | notif schemas edit order-placed

# Pipe workflow: get -> modify -> update
notif schemas get order-placed --schema | jq '.properties.amount.type = "integer"' | notif schemas edit order-placed

# Specify version explicitly
notif schemas edit order-placed --version 2.0.0 < schema.json
```

### Other Commands

```bash
notif schemas list                    # List all schemas
notif schemas get <name>              # Get schema details
notif schemas get <name> --schema     # Output JSON Schema only
notif schemas versions <name>         # List versions
notif schemas validate <name> <data>  # Validate data
notif schemas delete <name>           # Delete schema
```

## Schema Codegen

Generate typed code (TypeScript + Zod, Go structs) from notif.sh JSON Schemas.

### Quick Start

```bash
# Initialize config
notif schemas init

# Generate code for all schemas
notif schemas generate

# Preview without writing
notif schemas generate --dry-run
```

### Configuration (.notif.yaml)

```yaml
version: 1
output:
  typescript: ./src/generated/notif
  go: ./internal/notif/schemas
options:
  typescript:
    exports: named       # named | default
  go:
    package: schemas
    jsonTags: omitempty  # omitempty | required | none

# Generate ALL schemas from server
schemas: all

# Or list specific schemas
schemas:
  - order-placed
  - name: user-created
    languages: [typescript]  # Only TypeScript
  - name: payment
    file: ./schemas/payment.yaml  # From local file
```

### Generated Code

**TypeScript** (with Zod validators):
```typescript
import { z } from 'zod';

export const OrderPlacedSchema = z.object({
  orderId: z.string(),
  amount: z.number(),
});

export type OrderPlaced = z.infer<typeof OrderPlacedSchema>;

export function validateOrderPlaced(data: unknown): OrderPlaced {
  return OrderPlacedSchema.parse(data);
}

export function isOrderPlaced(data: unknown): data is OrderPlaced {
  return OrderPlacedSchema.safeParse(data).success;
}
```

**Go**:
```go
package schemas

type OrderPlaced struct {
    OrderID string  `json:"orderId"`
    Amount  float64 `json:"amount"`
}
```

## Browser Testing with agent-browser

Use `agent-browser` CLI for frontend automation and testing.

### Quick Reference

```bash
# Open page and get element refs
agent-browser open http://localhost:3000 --session test
agent-browser snapshot -i --session test    # Interactive elements with refs

# Interact with elements (use @ref from snapshot)
agent-browser click @e1 --session test
agent-browser fill @e2 "text" --session test
agent-browser type @e2 "text" --session test  # Doesn't clear first

# Get information
agent-browser get url --session test
agent-browser get title --session test
agent-browser get text @e1 --session test

# Screenshots
agent-browser screenshot /tmp/screen.png --session test
agent-browser screenshot /tmp/full.png --full --session test

# Run JavaScript
agent-browser eval "document.title" --session test

# Close browser
agent-browser close --session test
```

### Typical Testing Flow

1. Open page: `agent-browser open <url> --session <name>`
2. Take snapshot: `agent-browser snapshot -i --session <name>`
3. Interact using refs: `agent-browser click @e1 --session <name>`
4. Verify with screenshot: `agent-browser screenshot /tmp/test.png --session <name>`
5. Close: `agent-browser close --session <name>`

### Useful Options

- `--session <name>`: Isolated browser session (required for parallel testing)
- `--headed`: Show browser window (not headless)
- `-i, --interactive`: Only show interactive elements in snapshot
- `-f, --full`: Full page screenshot
