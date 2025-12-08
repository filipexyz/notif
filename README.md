# notif.sh

A notification center for Claude Code sessions. Push notifications from anywhere, review them in a desktop UI, and have approved ones injected into your next Claude interaction.

## Architecture

```
notif/
├── crates/
│   ├── notif-core/    # Shared library (db, models, config)
│   ├── notif-cli/     # CLI binary
│   ├── notif-server/  # HTTP server library (webhooks)
│   └── notif-ui/      # Tauri desktop app
```

## Installation

```bash
# Install CLI
cargo install --path crates/notif-cli

# Build desktop UI
cd crates/notif-ui
npm install
npm run tauri build

# Configure Claude Code hook
notif init
```

## Workflow

```
notif add "msg"  →  pending  →  [UI review]  →  approved  →  notif hook  →  delivered
                                    ↓
                                dismissed
```

1. **Add** notifications via CLI (status: `pending`)
2. **Review** in the desktop UI - approve or dismiss
3. **Hook** injects approved notifications into Claude (status: `delivered`)

## CLI Usage

```bash
# Add notifications (pending by default)
notif add "Remember to review PR #42"
notif add -p high "URGENT: Deploy to prod!"
notif add -p low "Update docs someday"
notif add -t work,urgent "Fix the production bug"

# Add with extended content (for longer details)
notif add "Build failed" -c "Error: test_auth failed at line 42..."

# Add and auto-approve (skip UI review)
notif add --approve "Inject this immediately"

# Read full notification content
notif read 16

# Approve/dismiss via CLI
notif approve 42        # Approve by ID
notif approve --all     # Approve all pending
notif dismiss 42        # Dismiss by ID
notif dismiss --all     # Dismiss all pending

# List notifications
notif ls                # List pending
notif ls -t work        # Filter by tag

# Clear delivered notifications
notif clear
```

## Desktop UI

The Tauri app (`notif-ui`) provides a visual interface to review pending notifications:

- View all pending notifications with priority indicators
- Approve or dismiss individual notifications
- Bulk approve/dismiss all
- Edit notification messages before approval
- Auto-refreshes every 2 seconds

Run in development:
```bash
cd crates/notif-ui
npm run tauri dev
```

## Commands

| Command | Description |
|---------|-------------|
| `notif add <message>` | Add a pending notification |
| `notif add --approve <message>` | Add and auto-approve |
| `notif add -p high <message>` | Add with priority (high/normal/low) |
| `notif add -t work,urgent <message>` | Add with tags |
| `notif add -c "content" <message>` | Add with extended content |
| `notif read <id>` | Read full notification content |
| `notif approve <id>` | Approve a notification |
| `notif approve --all` | Approve all pending |
| `notif dismiss <id>` | Dismiss a notification |
| `notif dismiss --all` | Dismiss all pending |
| `notif ls` | List pending notifications |
| `notif ls -t work` | List filtered by tag |
| `notif hook` | Hook mode (outputs approved notifications) |
| `notif clear` | Remove delivered notifications |
| `notif init` | Setup hook in `~/.claude/settings.json` |
| `notif init -t work` | Setup hook + create `.notif.json` |
| `notif server` | Start HTTP server for webhooks |
| `notif server --keygen` | Generate a new API key |

## Status Flow

| Status | Description |
|--------|-------------|
| `pending` | Awaiting review in UI |
| `approved` | User approved, ready for injection |
| `dismissed` | User dismissed, will not be injected |
| `delivered` | Already injected into Claude |

## Tags & Project Filtering

### Adding Tags

```bash
notif add -t work "Work notification"
notif add -t work,urgent "Urgent work notification"
```

### Project Configuration

Create a `.notif.json` in your project root to filter which notifications appear:

```bash
notif init -t work,myproject
```

This creates:

```json
{
  "tags": ["work", "myproject"],
  "mode": "include"
}
```

- **include mode**: Only show notifications WITH at least one matching tag
- **exclude mode**: Hide notifications WITH any matching tag
- **Tag-less notifications** always show (they're global)

## How It Works

1. `notif add` stores notifications in a local SQLite database (`pending` status)
2. Review notifications in the desktop UI, approve or dismiss them
3. A `UserPromptSubmit` hook fires before every message you send to Claude
4. The hook reads up to 3 approved notifications (high priority first)
5. If `.notif.json` exists, notifications are filtered by tags
6. Approved notifications are injected as context that Claude sees
7. Delivered notifications are marked so they don't repeat

## Hook Output

When you have approved notifications, Claude sees:

```
Pending notifications:
- [work, urgent] Fix the production bug
- [work] Review PR #42
- Remember to update README
```

## Use Cases

- **CI/CD**: `notif add -p high -t ci "Build failed!"` from your pipeline
- **Per-project context**: Different projects see different notifications via `.notif.json`
- **Cron jobs**: Daily reminders or scheduled notes
- **Long-running tasks**: Notify yourself when a process completes
- **Team workflows**: Tag notifications by project or team
- **Immediate injection**: Use `--approve` to bypass UI review

## Configuration

### Global Hook

The hook is configured in `~/.claude/settings.json`:

```json
{
  "hooks": {
    "UserPromptSubmit": [{
      "hooks": [{
        "type": "command",
        "command": "notif hook"
      }]
    }]
  }
}
```

### Project Filter

Optional `.notif.json` in project root:

```json
{
  "tags": ["work", "myproject"],
  "mode": "include"
}
```

## Remote Server (Webhooks)

Run an HTTP server to receive notifications from external services via webhooks.

### Start Server

```bash
# Start with default settings
notif server

# Custom host/port
notif server --host 0.0.0.0 --port 8787

# Generate API key
notif server --keygen
```

### Configuration

Environment variables:
- `NOTIF_API_KEY` - API key for authentication
- `NOTIF_SERVER_HOST` - Host to bind (default: 127.0.0.1)
- `NOTIF_SERVER_PORT` - Port to listen on (default: 8787)

Or create `~/.notif/server.toml`:

```toml
[server]
host = "0.0.0.0"
port = 8787

[auth]
api_key = "notif_your_key_here"
```

### API Endpoints

All endpoints require `X-API-Key` header.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/notifications` | Create notification |
| GET | `/notifications` | List (with `?status=pending`) |
| GET | `/notifications/{id}` | Get single notification |
| PUT | `/notifications/{id}/approve` | Approve |
| PUT | `/notifications/{id}/dismiss` | Dismiss |
| DELETE | `/notifications/{id}` | Delete |
| POST | `/notifications/approve-all` | Approve all pending |
| POST | `/notifications/dismiss-all` | Dismiss all pending |

### Create Notification

```bash
curl -X POST http://localhost:8787/notifications \
  -H "Content-Type: application/json" \
  -H "X-API-Key: notif_xxx" \
  -d '{"message":"Build passed!","priority":"high","tags":["ci","build"]}'
```

Request body:
```json
{
  "message": "Build passed!",
  "priority": "high",
  "tags": ["ci", "build"],
  "auto_approve": false,
  "content": "Optional extended content with more details..."
}
```

Response:
```json
{
  "success": true,
  "data": { "id": 42 },
  "error": null
}
```

## Docker (Server Hosting)

Deploy the notification server with Docker:

```bash
# Build and run with docker-compose
NOTIF_API_KEY=notif_your_secret_key docker-compose up -d

# Or run directly
docker run -d -p 8787:8787 \
  -e NOTIF_API_KEY=notif_your_secret_key \
  -v notif-data:/data \
  filipeai/notif:latest

# Build locally
docker build -t filipeai/notif:latest .

# Generate an API key
docker run --rm filipeai/notif:latest notif server --keygen
```

## Development

```bash
# Build everything
cargo build

# Run CLI
cargo run -p notif -- add "test"

# Run UI in dev mode
cd crates/notif-ui && npm run tauri dev

# Build release
cargo build --release
cd crates/notif-ui && npm run tauri build
```

## License

MIT
