# Claude Code Workers

Run Claude Code agents over notif.sh pub/sub with automatic discovery.

## Requirements

```bash
pip install notifsh rich
```

## Usage

### Start a Worker

```bash
python worker.py <agent-name> [options]
```

Options:
- `--budget, -b` - Max budget in USD (unlimited if not set)
- `--cwd, -C` - Working directory for Claude
- `--server, -s` - Notif server URL (default: https://api.notif.sh)

Example:
```bash
python worker.py coder
python worker.py coder --budget 5 --cwd ~/projects/myapp
```

### Run the Client

```bash
python client.py
```

The client will:
1. Discover available workers
2. Display them in a table
3. Let you select one and start chatting
4. Sessions persist across messages

## Protocol

| Topic | Direction | Purpose |
|-------|-----------|---------|
| `claude.agents.discover` | Client → Workers | Discovery broadcast |
| `claude.agents.available` | Workers → Client | Announce availability |
| `claude.trigger.<agent>` | Client → Worker | Send prompt |
| `claude.response.<agent>` | Worker → Client | Return result |

## Message Format

**Trigger:**
```json
{
  "prompt": "What files are here?",
  "session": "uuid-for-continuation",
  "request_id": "req_abc123"
}
```

**Response:**
```json
{
  "request_id": "req_abc123",
  "result": "Here are the files...",
  "session": "uuid-for-continuation",
  "is_error": false,
  "cost_usd": 0.05
}
```
