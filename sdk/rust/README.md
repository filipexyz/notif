# notifsh

Rust SDK for [notif.sh](https://notif.sh) - a managed pub/sub event hub with webhooks, DLQ, and real-time subscriptions.

## Installation

Add to your `Cargo.toml`:

```toml
[dependencies]
notifsh = "0.1"
tokio = { version = "1", features = ["rt-multi-thread", "macros"] }
serde_json = "1"
futures = "0.3"
```

## Quick Start

```rust
use notifsh::Notif;
use serde_json::json;
use futures::StreamExt;

#[tokio::main]
async fn main() -> notifsh::Result<()> {
    let client = Notif::from_env()?;

    // Emit an event
    let response = client.emit("orders.created", json!({"order_id": "123"})).await?;
    println!("Published: {}", response.id);

    // Subscribe to events
    let mut stream = client.subscribe(&["orders.*"]).await?;
    while let Some(event) = stream.next().await {
        let event = event?;
        println!("{}: {:?}", event.topic, event.data);
    }

    Ok(())
}
```

## Configuration

### From Environment

```rust
// Reads NOTIF_API_KEY environment variable
let client = Notif::from_env()?;
```

### With Builder

```rust
use std::time::Duration;

let client = Notif::builder("nsh_your_api_key")
    .server("http://localhost:8080")
    .timeout(Duration::from_secs(60))
    .build()?;
```

## Emitting Events

```rust
use serde_json::json;

let response = client.emit("orders.created", json!({
    "order_id": "ord_123",
    "customer": "john@example.com",
    "total": 99.99
})).await?;

println!("Event ID: {}", response.id);
```

## Subscribing to Events

### Simple Subscription

```rust
use futures::StreamExt;

let mut stream = client.subscribe(&["orders.*"]).await?;

while let Some(event) = stream.next().await {
    let event = event?;
    println!("{}: {:?}", event.topic, event.data);
}
```

### With Options

```rust
use notifsh::SubscribeOptions;

let mut stream = client
    .subscribe_with_options(
        &["orders.*", "users.*"],
        SubscribeOptions::new()
            .auto_ack(false)          // Manual acknowledgment
            .from("beginning")        // Start from oldest events
            .group("worker-pool"),    // Consumer group for load balancing
    )
    .await?;

while let Some(event) = stream.next().await {
    let event = event?;

    // Process event...

    // Acknowledge success
    event.ack().await?;

    // Or negative acknowledge for retry
    // event.nack(Some("5m")).await?;
}
```

### Subscribe Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `auto_ack` | `bool` | `true` | Automatically acknowledge events |
| `from` | `Option<String>` | `None` | Start position: "latest", "beginning", or ISO8601 timestamp |
| `group` | `Option<String>` | `None` | Consumer group name for load balancing |

## Error Handling

```rust
use notifsh::NotifError;

match client.emit("topic", json!({})).await {
    Ok(response) => println!("Success: {}", response.id),
    Err(NotifError::Auth(msg)) => eprintln!("Auth error: {}", msg),
    Err(NotifError::Api { status, message }) => eprintln!("API error {}: {}", status, message),
    Err(NotifError::Connection(msg)) => eprintln!("Connection error: {}", msg),
    Err(e) => eprintln!("Other error: {}", e),
}
```

## Examples

Run the examples:

```bash
export NOTIF_API_KEY=nsh_your_api_key

# Emit an event
cargo run --example emit

# Subscribe to events
cargo run --example subscribe
```

## License

MIT
