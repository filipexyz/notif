# notif.sh

TypeScript SDK for [notif.sh](https://notif.sh) event hub.

## Installation

```bash
npm install notif.sh
```

## Quick Start

```typescript
import { Notif } from 'notif.sh'

// Uses NOTIF_API_KEY env var by default
const n = new Notif()

// Emit an event
await n.emit('leads.new', { name: 'John', email: 'john@example.com' })

// Subscribe to events (auto-ack by default)
for await (const event of n.subscribe('leads.*')) {
  console.log(`Received: ${event.topic} - ${JSON.stringify(event.data)}`)
}

n.close()
```

## Configuration

```typescript
import { Notif } from 'notif.sh'

// Using environment variable (recommended)
// Set NOTIF_API_KEY=nsh_your_api_key
const client = new Notif()

// Or pass API key directly
const client = new Notif({ apiKey: 'nsh_your_api_key' })

// With custom server
const client = new Notif({ server: 'http://localhost:8080' })

// With custom timeout (ms)
const client = new Notif({ timeout: 60000 })
```

## Singleton Pattern

For applications that need a single shared instance (similar to Prisma), create your own singleton:

```typescript
// lib/notif.ts
import { Notif } from 'notif.sh'

// Prevent multiple instances in development (hot reload)
const globalForNotif = globalThis as unknown as { notif: Notif }

export const notif = globalForNotif.notif || new Notif()

if (process.env.NODE_ENV !== 'production') {
  globalForNotif.notif = notif
}
```

Then import from your singleton:

```typescript
import { notif } from './lib/notif'

await notif.emit('orders.created', { id: '123' })
```

**When to use singleton:**
- Long-running servers that subscribe to events
- Applications where multiple components emit events
- Avoiding connection overhead per request

**When to create new instances:**
- Different API keys per tenant
- Isolated testing
- Short-lived scripts

## Emitting Events

```typescript
const n = new Notif()

const result = await n.emit('orders.created', {
  orderId: '12345',
  amount: 99.99,
})

console.log(`Event ID: ${result.id}`)

n.close()
```

## Subscribing to Events

```typescript
const n = new Notif()

// Subscribe to multiple topics
for await (const event of n.subscribe('orders.*', 'payments.*')) {
  console.log(`${event.topic}: ${JSON.stringify(event.data)}`)
}

// Manual acknowledgment
for await (const event of n.subscribe('orders.*', { autoAck: false })) {
  try {
    await process(event.data)
    await event.ack()
  } catch (err) {
    await event.nack('5m') // Retry in 5 minutes
  }
}

// Consumer groups (load-balanced)
for await (const event of n.subscribe('jobs.*', { group: 'worker-pool' })) {
  await processJob(event.data)
}

// Start from beginning
for await (const event of n.subscribe('orders.*', { from: 'beginning' })) {
  console.log(event.data)
}

n.close()
```

## Error Handling

```typescript
import { Notif, AuthError, APIError, ConnectionError } from 'notif.sh'

try {
  const n = new Notif()
  await n.emit('test', { data: 'value' })
  n.close()
} catch (err) {
  if (err instanceof AuthError) {
    console.log('Invalid API key')
  } else if (err instanceof APIError) {
    console.log(`API error (${err.statusCode}): ${err.message}`)
  } else if (err instanceof ConnectionError) {
    console.log(`Connection failed: ${err.message}`)
  }
}
```

## Requirements

- Node.js 18+

## License

MIT
