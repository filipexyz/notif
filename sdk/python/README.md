# notifsh

Python SDK for [notif.sh](https://notif.sh) event hub.

## Installation

```bash
pip install notifsh
```

## Quick Start

```python
import asyncio
from notifsh import Notif

async def main():
    # Uses NOTIF_API_KEY env var by default
    async with Notif() as n:
        # Emit an event
        await n.emit('leads.new', {'name': 'John', 'email': 'john@example.com'})

        # Subscribe to events (auto-ack by default)
        async for event in n.subscribe('leads.*'):
            print(f"Received: {event.topic} - {event.data}")

asyncio.run(main())
```

## Configuration

```python
from notifsh import Notif

# Using environment variable (recommended)
# Set NOTIF_API_KEY=nsh_your_api_key
client = Notif()

# Or pass API key directly
client = Notif(api_key='nsh_your_api_key')

# With custom server
client = Notif(server='http://localhost:8080')
```

## Emitting Events

```python
async with Notif() as n:
    result = await n.emit('orders.created', {
        'order_id': '12345',
        'amount': 99.99,
    })
    print(f"Event ID: {result.id}")
```

## Subscribing to Events

```python
async with Notif() as n:
    # Subscribe to multiple topics
    async for event in n.subscribe('orders.*', 'payments.*'):
        print(f"{event.topic}: {event.data}")

    # Manual acknowledgment
    async for event in n.subscribe('orders.*', auto_ack=False):
        try:
            process(event.data)
            await event.ack()
        except Exception:
            await event.nack('5m')  # Retry in 5 minutes

    # Consumer groups (load-balanced)
    async for event in n.subscribe('jobs.*', group='worker-pool'):
        await process_job(event.data)

    # Start from beginning
    async for event in n.subscribe('orders.*', from_='beginning'):
        print(event.data)
```

## Error Handling

```python
from notifsh import Notif, AuthError, APIError, ConnectionError

try:
    async with Notif() as n:
        await n.emit('test', {'data': 'value'})
except AuthError:
    print("Invalid API key")
except APIError as e:
    print(f"API error ({e.status_code}): {e}")
except ConnectionError as e:
    print(f"Connection failed: {e}")
```

## Requirements

- Python 3.11+
- httpx
- websockets

## License

MIT
