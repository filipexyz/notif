# n8n-nodes-notif

n8n community nodes for [notif.sh](https://notif.sh) event hub.

## Installation

### Community Nodes (Recommended)

1. Go to **Settings > Community Nodes**
2. Select **Install**
3. Enter `n8n-nodes-notif` and click **Install**

### Manual Installation

```bash
cd ~/.n8n/nodes
npm install n8n-nodes-notif
```

## Nodes

### Notif

Emit events to notif.sh from your workflow.

**Configuration:**
- **Topic**: The topic to emit the event to (e.g., `orders.new`)
- **Data**: The event data as JSON

### Notif Trigger

Starts a workflow when events are received from notif.sh.

**Features:**
- Automatically creates a webhook when the workflow is activated
- Automatically deletes the webhook when the workflow is deactivated
- Verifies HMAC signatures for security
- Supports topic wildcards (`*` for single segment, `>` for all remaining)

**Configuration:**
- **Topics**: Comma-separated topic patterns (e.g., `orders.*`, `leads.new`)

## Credentials

### Notif API

- **API Key**: Your notif.sh API key (starts with `nsh_`)
- **Server URL**: API server URL (default: `https://api.notif.sh`)

Get your API key at [app.notif.sh](https://app.notif.sh).

## Example Workflow

1. Add a **Notif Trigger** node
2. Configure topics: `orders.*, payments.completed`
3. Activate the workflow
4. Events matching those topics will trigger the workflow

## Topic Patterns

| Pattern | Matches |
|---------|---------|
| `orders.new` | Exactly `orders.new` |
| `orders.*` | `orders.new`, `orders.updated` (single segment) |
| `orders.>` | `orders.new`, `orders.us.created` (all remaining) |

## Resources

- [notif.sh Documentation](https://docs.notif.sh)
- [n8n Community Nodes](https://docs.n8n.io/integrations/community-nodes/)

## License

MIT
