# notif.sh — Product Requirements Document

## Vision

A simple event hub for AI agents and automations. Centralizes events from any source (n8n, scripts, crons, APIs) and distributes to any consumer (agents, webhooks, other systems).

**One-liner:** "Managed pub/sub for devs who don't want to manage infra."

---

## Problem

Today, to connect events between systems, devs need to:

1. Spin up Redis/NATS/Kafka and manage it
2. Implement retry, dead letter, manual ack
3. Build their own API to expose it
4. Build a dashboard to debug

Or use direct webhooks, which:
- Lose messages if destination is offline
- No built-in retry
- Each integration is a new endpoint

**Specific pain for AI agents:**

Agents need to receive commands, emit status, request approval. Today each one implements its own event system, or polls APIs.

---

## Solution

notif.sh is a managed event hub with:

- **Topics** — named channels (`leads.new`, `agent.stuck`, `deploy.prod`)
- **Publish** — HTTP POST from anywhere
- **Subscribe** — WebSocket or polling with ack/nack
- **Persistence** — messages don't get lost
- **Auto retry** — with configurable backoff
- **Dead letter** — events that failed too many times go to separate queue

---

## Target Audience

1. **Devs building AI agents** — want "task.assigned" event to reach the agent
2. **n8n/Make/Zapier users** — want reliable triggers between workflows
3. **Small teams** — don't want to manage Kafka for 1000 msgs/day

---

## Principles

1. **Simple first** — works with a curl, no SDK required
2. **Reliable** — messages don't get lost
3. **Cheap** — generous free tier, pay when you scale
4. **No vendor lock-in** — easy export, documented protocol

---

## Core Features (MVP)

### 1. Topics

Named channels with wildcard support.

```
leads.new
leads.qualified  
agent.research.started
agent.research.done
agent.*.error        # wildcard
```

Topics are created implicitly on first publish.

### 2. Publish (Emit)

```bash
# HTTP
curl -X POST https://api.notif.sh/emit \
  -H "Authorization: Bearer nsh_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "leads.new",
    "data": {"name": "John", "email": "j@test.com"}
  }'

# Response
{
  "id": "evt_abc123",
  "topic": "leads.new", 
  "created_at": "2025-01-15T10:30:00Z"
}
```

### 3. Subscribe

**WebSocket (real-time):**

```javascript
const ws = new WebSocket("wss://api.notif.sh/subscribe?token=nsh_xxx")

ws.send(JSON.stringify({
  action: "subscribe",
  topics: ["leads.*"],
  options: {
    auto_ack: false,
    from: "beginning"  // or "latest", or timestamp
  }
}))

ws.onmessage = (msg) => {
  const event = JSON.parse(msg.data)
  // { id: "evt_abc", topic: "leads.new", data: {...}, timestamp: "..." }
  
  // Process...
  
  ws.send(JSON.stringify({ action: "ack", id: event.id }))
  // or: { action: "nack", id: event.id, retry_in: "5m" }
}
```

**HTTP Polling (simple):**

```bash
# Get next events (long-poll, up to 30s)
GET /subscribe?topics=leads.*&timeout=30s

# Response
{
  "events": [
    {"id": "evt_abc", "topic": "leads.new", "data": {...}}
  ]
}

# Confirm processing
POST /ack
{"ids": ["evt_abc", "evt_def"]}
```

### 4. Acknowledgment

| Mode | Behavior |
|------|----------|
| `auto_ack: true` | Auto ack when event is delivered |
| `auto_ack: false` | Consumer must call explicit ack/nack |

**Retry with backoff:**

```javascript
{
  action: "nack",
  id: "evt_abc",
  retry_in: "5m"  // or "30s", "1h", etc
}
```

**Retry config per subscription:**

```javascript
{
  action: "subscribe",
  topics: ["leads.*"],
  options: {
    auto_ack: false,
    max_retries: 5,
    retry_delays: ["10s", "1m", "5m", "30m", "2h"],
    ack_timeout: "5m"  // if no ack in 5min, auto retry
  }
}
```

### 5. Dead Letter Queue

Events that exceeded `max_retries` go to topic `$dlq.<original_topic>`:

```javascript
// Subscribe to dead letter
{
  action: "subscribe", 
  topics: ["$dlq.leads.*"]
}
```

### 6. Consumer Groups

Multiple instances of the same consumer, only one receives each event:

```javascript
{
  action: "subscribe",
  topics: ["leads.*"],
  group: "lead-processor"  // load balance among consumers in group
}
```

Without group = each consumer receives all events (fan-out).

### 7. Authentication

Simple API key authentication via `Authorization: Bearer` header.

**Key format:**

```
nsh_live_abc123xyz
│    │    │
│    │    └── random string (24 chars)
│    └── environment (live/test)
└── prefix (notif.sh)
```

**Environments:**

| Environment | Prefix | Purpose |
|-------------|--------|---------|
| Live | `nsh_live_` | Production traffic |
| Test | `nsh_test_` | Development/testing, isolated data |

**Usage:**

```bash
# HTTP Header
Authorization: Bearer nsh_live_abc123xyz

# WebSocket query param
wss://api.notif.sh/subscribe?token=nsh_live_abc123xyz
```

Test keys have separate quotas and don't affect production data.

---

## SDKs

### Python

```python
from notif import Notif

n = Notif("nsh_xxx")

# Emit
n.emit("leads.new", {"name": "John"})

# Subscribe with decorator
@n.on("leads.*")
def handle_lead(event):
    print(event.topic, event.data)
    # auto-ack on return

@n.on("orders.*", auto_ack=False, group="order-processor")
def handle_order(event):
    try:
        process(event.data)
        event.ack()
    except TemporaryError:
        event.nack(retry_in="5m")
    except PermanentError:
        event.ack()  # don't retry, but log

# Start listening (blocking)
n.listen()

# Or async
async def main():
    await n.listen_async()
```

### JavaScript/TypeScript

```typescript
import { Notif } from 'notif.sh'

const n = new Notif('nsh_xxx')

// Emit
await n.emit('leads.new', { name: 'John' })

// Subscribe
n.on('leads.*', async (event) => {
  console.log(event.topic, event.data)
})

// Manual ack
n.on('orders.*', { autoAck: false }, async (event) => {
  try {
    await process(event.data)
    await event.ack()
  } catch (e) {
    await event.nack({ retryIn: '5m' })
  }
})

// Listen
await n.listen()
```

### CLI

```bash
# Install
curl -fsSL https://notif.sh/install.sh | sh

# Configure
notif auth nsh_xxx

# Emit
notif emit leads.new '{"name": "John"}'

# Subscribe (stdout)
notif subscribe "leads.*"
# evt_abc | leads.new | {"name": "John"}
# evt_def | leads.qualified | {"name": "Mary"}

# Subscribe with command (executes for each event)
notif subscribe "leads.*" --exec "python process.py"

# Manual ack
notif ack evt_abc

# View pending events
notif pending leads.*

# View dead letter
notif dlq leads.*
```

---

## Integrations

### n8n Node

```
┌─────────────────────────────────┐
│  notif.sh Trigger               │
│                                 │
│  Topic: leads.new               │
│  Auto-ack: ✓                    │
└─────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────┐
│  Process Lead                   │
└─────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────┐
│  notif.sh Emit                  │
│                                 │
│  Topic: leads.processed         │
│  Data: {{ $json }}              │
└─────────────────────────────────┘
```

### Webhook Receiver

Receives external webhooks and converts to events:

```bash
POST https://api.notif.sh/webhook/github?token=nsh_xxx

# Becomes event at: github.push, github.pr.opened, etc
```

### Outbound Webhooks

Sends events to external URLs:

```bash
POST /webhooks
{
  "topics": ["leads.*"],
  "url": "https://my-api.com/hook",
  "headers": {"X-Secret": "xxx"}
}
```

---

## Dashboard

### Events View

- List of recent events
- Filter by topic, status, date
- Event detail (data, retries, timestamps)
- Manual event replay

### Topics View

- Active topics
- Throughput per topic
- Connected consumers

### Dead Letter View

- Failed events
- Failure reason
- Replay or discard

### Settings

- API keys
- Webhook endpoints
- Consumer groups
- Retention config

---

## Technical Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         notif.sh                            │
│                                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ REST API │  │WebSocket │  │Dashboard │  │ Webhook  │   │
│  │  /emit   │  │/subscribe│  │  (React) │  │ Receiver │   │
│  └────┬─────┘  └────┬─────┘  └──────────┘  └────┬─────┘   │
│       │             │                           │          │
│       └─────────────┼───────────────────────────┘          │
│                     ▼                                       │
│              ┌─────────────┐                                │
│              │   Router    │                                │
│              │ (auth, rate │                                │
│              │   limit)    │                                │
│              └──────┬──────┘                                │
│                     ▼                                       │
│              ┌─────────────┐      ┌─────────────┐          │
│              │    NATS     │      │  Postgres   │          │
│              │ (JetStream) │      │ (metadata,  │          │
│              │             │      │  api keys)  │          │
│              └─────────────┘      └─────────────┘          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Stack

| Component | Tech |
|-----------|------|
| API Server | Go |
| Messaging | NATS JetStream |
| Metadata | Postgres |
| Dashboard | React + Vite |
| SDKs | Python, TypeScript, Go |

### Why NATS?

- Native persistence (JetStream)
- Native consumer groups
- Topic wildcards
- Built-in ack/nack
- Lightweight (~20MB)
- Easy to operate

---

## Pricing (Future)

| Tier | Price | Events/month | Retention | Consumers |
|------|-------|--------------|-----------|-----------|
| Free | $0 | 10k | 24h | 3 |
| Pro | $29 | 500k | 7 days | 20 |
| Team | $99 | 5M | 30 days | Unlimited |
| Enterprise | Custom | Unlimited | Custom | Custom |

---

## Roadmap

### MVP (v0.1)

- [ ] API: emit, subscribe (WebSocket), ack/nack
- [ ] NATS JetStream integration
- [ ] API key auth
- [ ] Basic CLI
- [ ] Python SDK
- [ ] Single-node deploy

### v0.2

- [ ] Basic dashboard
- [ ] Consumer groups
- [ ] Dead letter queue
- [ ] TypeScript SDK
- [ ] HTTP polling subscribe

### v0.3

- [ ] Webhook receiver (GitHub, Stripe, etc)
- [ ] Outbound webhooks
- [ ] n8n node
- [ ] Event replay
- [ ] Basic metrics

### v1.0

- [ ] Multi-tenancy
- [ ] Team management
- [ ] Billing integration
- [ ] 99.9% SLA

---

## Success Metrics

| Metric | MVP Target |
|--------|------------|
| p99 Latency | < 100ms |
| Uptime | 99% |
| Events/sec (single node) | 10k |

---

## Risks

| Risk | Mitigation |
|------|------------|
| NATS doesn't scale | Start single-node, cluster later |
| Infra costs | Events are cheap, charge for retention |
| Competition (Upstash, etc) | Focus on DX and AI use case |

---

## Open Questions

1. **Self-hosted?** Offer Docker image for those who want to run their own?
2. **Bridges?** Add bridges (Slack, Discord) or keep pure events?
3. **Schema validation?** Validate event payloads or keep it free-form?
4. **Ordering guarantees?** FIFO per topic or best-effort?

---

## References

- [NATS JetStream](https://docs.nats.io/nats-concepts/jetstream)
- [Upstash Kafka](https://upstash.com/kafka)
- [Inngest](https://inngest.com)
- [Trigger.dev](https://trigger.dev)
