# Modular Desktop

A modular desktop notification system where each window is a separate Tauri binary, all connected via notif.sh pub/sub.

## Architecture

```
┌─────────────┐     notif.sh      ┌─────────────┐
│     Hub     │ ───────────────── │    Toast    │
│  (emitter)  │    pub/sub        │ (subscriber)│
└─────────────┘                   └─────────────┘
       │                                 │
       │         ┌─────────────┐         │
       └──────── │  Event Log  │ ────────┘
                 │ (subscriber)│
                 └─────────────┘

Claude Code Integration:
┌─────────────┐  permission.request  ┌─────────────┐
│ Claude Code │ ──────────────────── │    Toast    │
│   (hook)    │ ◄─────────────────── │  (approve)  │
└─────────────┘  permission.response └─────────────┘
```

## Components

| App | Description | Topics |
|-----|-------------|--------|
| **hub** | Control panel - sends notifications | Emits `desktop.hub.notify` |
| **toast** | Transparent popup widget | Subscribes to `desktop.hub.notify`, `claude.permission.request` |
| **event-log** | Real-time event viewer | Subscribes to `desktop.>` (all) |

## Prerequisites

- Rust 1.70+
- [Tauri prerequisites](https://tauri.app/v1/guides/getting-started/prerequisites)
- A notif.sh API key

## Setup

1. Set your API key:
```bash
export NOTIF_API_KEY=nsh_your_api_key
```

2. Build all apps:
```bash
cd examples/modular-desktop
cargo build --release
```

## Running

Start each app (requires API key in environment):

```bash
# Start all apps
./target/release/hub &
./target/release/toast &
./target/release/event-log &
```

Or in separate terminals for debugging.

## How It Works

1. **Hub** sends a notification via `desktop.hub.notify`
2. **Toast** receives the event via WebSocket subscription and shows a popup
3. **Event Log** shows all `desktop.*` events in a scrollable timeline

Each app is a standalone Tauri binary. They don't share memory or state - all communication happens through notif.sh.

## SDK Usage

All apps use the `notifsh` Rust SDK:

```rust
use notifsh::{Notif, SubscribeOptions};
use futures_util::StreamExt;

// Emit an event
let client = Notif::from_env()?;
client.emit("desktop.hub.notify", json!({"title": "Hello"})).await?;

// Subscribe to events
let mut stream = client
    .subscribe_with_options(
        &["desktop.>"],
        SubscribeOptions::new().auto_ack(true).from("latest"),
    )
    .await?;

while let Some(event) = stream.next().await {
    let event = event?;
    println!("Got: {} - {:?}", event.topic, event.data);
}
```

## CLI Testing

Emit events from the CLI:

```bash
notif emit desktop.hub.notify '{"id":"1","title":"Test","body":"Hello","level":"info"}'
```

## Claude Code Permission Hook

Toast can handle Claude Code permission requests, showing a native popup for Allow/Deny decisions.

### Setup

Add to your Claude Code settings (`~/.claude/settings.json`):

```json
{
  "hooks": {
    "PermissionRequest": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "notif emit claude.permission.request --reply-to claude.permission.response --timeout 60s --json --raw"
          }
        ]
      }
    ]
  }
}
```

### How It Works

1. Claude Code triggers a permission request (e.g., running a Bash command)
2. The hook emits the request to `claude.permission.request` via notif CLI
3. Toast receives the event and shows a popup with tool name and input
4. User clicks Allow or Deny
5. Toast emits the decision to `claude.permission.response`
6. The CLI receives the response and returns it to Claude Code

### Topics

| Topic | Direction | Description |
|-------|-----------|-------------|
| `claude.permission.request` | CLI → Toast | Permission request with tool details |
| `claude.permission.response` | Toast → CLI | User decision (allow/deny) |

### Testing

```bash
# Test the permission flow
notif emit claude.permission.request \
  '{"tool_name": "Bash", "tool_input": {"command": "ls -la"}}' \
  --reply-to claude.permission.response \
  --timeout 60s \
  --raw
```

Click Allow/Deny in Toast to see the response.

## Project Structure

```
modular-desktop/
├── Cargo.toml          # Workspace manifest
├── hub/                # Notification sender
│   ├── src/lib.rs
│   ├── ui/index.html
│   └── tauri.conf.json
├── toast/              # Popup notifications (transparent window)
│   ├── src/lib.rs
│   ├── ui/index.html
│   └── tauri.conf.json
└── event-log/          # Event viewer
    ├── src/lib.rs
    ├── ui/index.html
    └── tauri.conf.json
```

## Extending

Add new widgets by:

1. Create a new crate in the workspace
2. Use the `notifsh` SDK for pub/sub
3. Subscribe to relevant topics
4. Add to the workspace in `Cargo.toml`

Example topics for new widgets:
- `desktop.settings.*` - Settings widget
- `desktop.status.*` - Status bar widget
- `desktop.chat.*` - Chat widget
