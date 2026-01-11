# notif.sh

**Managed pub/sub for devs who don't want to manage infra.**

notif.sh is an event hub that centralizes events from any source and reliably delivers them to any consumer. Built for AI agents, automations, and integrations.

## Quick Start

### 1. Get an API Key

Sign up at [app.notif.sh](https://app.notif.sh) to get your API key.

### 2. Install

**CLI (Unix)**
```bash
curl -fsSL https://notif.sh/install.sh | sh
notif auth <your-api-key>
```

**TypeScript / Node.js**
```bash
npm install notif.sh
```

**Python**
```bash
pip install notifsh
```

**Rust**
```bash
cargo add notifsh
```

### 3. Publish Events

**CLI**
```bash
notif emit orders.new '{"order_id": "12345", "amount": 99.99}'
```

**TypeScript**
```typescript
import { Notif } from 'notif.sh'

const n = new Notif()  // Uses NOTIF_API_KEY env var
await n.emit('orders.new', { order_id: '12345', amount: 99.99 })
```

**Python**
```python
from notifsh import Notif

async with Notif() as n:
    await n.emit('orders.new', {'order_id': '12345', 'amount': 99.99})
```

**Rust**
```rust
use notifsh::Notif;
use serde_json::json;

let client = Notif::from_env()?;
client.emit("orders.new", json!({"order_id": "12345", "amount": 99.99})).await?;
```

### 4. Request-Response (CLI)

Emit and wait for a response on another topic:

```bash
notif emit 'tasks.create' '{"task_id": "abc", "action": "process"}' \
  --reply-to 'tasks.completed,tasks.failed' \
  --filter '.task_id == "abc"' \
  --timeout 60s
```

This subscribes to reply topics, emits the event, and waits for a matching response.

### 5. Subscribe to Events

**CLI**
```bash
notif subscribe 'orders.*'
```

**TypeScript**
```typescript
for await (const event of n.subscribe('orders.*')) {
  console.log(event.topic, event.data)
}
```

**Python**
```python
async for event in n.subscribe('orders.*'):
    print(event.topic, event.data)
```

**Rust**
```rust
use futures::StreamExt;

let mut stream = client.subscribe(&["orders.*"]).await?;
while let Some(event) = stream.next().await {
    let event = event?;
    println!("{} {:?}", event.topic, event.data);
}
```

## Features

| Feature | Description |
|---------|-------------|
| **Topics** | Named channels with wildcards (`leads.*`, `orders.>`) |
| **Publish** | Simple HTTP POST or SDK |
| **Subscribe** | Real-time WebSocket with ack/nack |
| **Webhooks** | HTTP delivery with HMAC signing |
| **Auto-retry** | Exponential backoff with max retries |
| **Dead Letter Queue** | Failed events preserved for replay |
| **Consumer Groups** | Load-balance across instances |

## Topic Patterns

| Pattern | Matches |
|---------|---------|
| `leads.new` | Exactly `leads.new` |
| `leads.*` | `leads.new`, `leads.qualified` (single segment) |
| `orders.>` | `orders.new`, `orders.us.paid` (all remaining) |

## Manual Acknowledgment

For at-least-once delivery, disable auto-ack and manually acknowledge:

**TypeScript**
```typescript
for await (const event of n.subscribe('orders.*', { autoAck: false })) {
  try {
    await processOrder(event.data)
    await event.ack()
  } catch (err) {
    await event.nack('5m')  // Retry in 5 minutes
  }
}
```

**Python**
```python
async for event in n.subscribe('orders.*', auto_ack=False):
    try:
        await process_order(event.data)
        await event.ack()
    except Exception:
        await event.nack('5m')
```

**Rust**
```rust
use notifsh::SubscribeOptions;

let mut stream = client
    .subscribe_with_options(&["orders.*"], SubscribeOptions::new().auto_ack(false))
    .await?;

while let Some(event) = stream.next().await {
    let event = event?;
    match process_order(&event.data).await {
        Ok(_) => event.ack().await?,
        Err(_) => event.nack(Some("5m")).await?,
    }
}
```

## REST API

Base URL: `https://api.notif.sh`

### Publish

```bash
curl -X POST https://api.notif.sh/api/v1/emit \
  -H "Authorization: Bearer $NOTIF_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"topic": "leads.new", "data": {"name": "John"}}'
```

### Webhooks

```bash
# Create webhook
curl -X POST https://api.notif.sh/api/v1/webhooks \
  -H "Authorization: Bearer $NOTIF_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://my-api.com/hook", "topics": ["leads.*"]}'
```

## Links

- [Dashboard](https://app.notif.sh)
- [Documentation](https://docs.notif.sh)
- [TypeScript SDK](https://www.npmjs.com/package/notif.sh)
- [Python SDK](https://pypi.org/project/notifsh)
- [Rust SDK](https://crates.io/crates/notifsh)
- [GitHub Repository](https://github.com/filipexyz/notif)

## Self-Hosting

notif.sh is open source. See [CONTRIBUTING.md](CONTRIBUTING.md) for self-hosting and development instructions.

## License

MIT
