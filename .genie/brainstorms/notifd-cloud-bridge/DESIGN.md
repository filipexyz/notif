# Design: `notif connect`

> Bridge any local NATS to notif.sh cloud via NATS leaf node protocol.

## Overview

`notif connect` is a subcommand of the existing `notif` CLI that connects a local
NATS server to the notif.sh cloud via NATS leaf node protocol. Runs interceptors
locally (jq transforms), and bridges events bidirectionally at the protocol level.
Runs as a system service (systemd/launchd) via `kardianos/service`.

No Postgres. No HTTP server. No HTTP/WS bridge. Just NATS.

## Delivery Phases

**Phase 1 (independent, no dependencies):**
- `notif connect` local-only mode
- Interceptors (jq transforms) on local NATS
- Service lifecycle (install/start/stop/status/logs/uninstall)
- No `--cloud` flag yet

**Phase 2 (after nats-accounts ships):**
- `--cloud` flag enabled
- Credential provisioning (NKey from notif.sh API)
- Leaf node configuration (patch existing NATS config)
- Bidirectional cloud sync

Phase 1 delivers value immediately (local interceptors) without any cloud
dependency. Phase 2 activates when NATS Accounts exist in cloud.

## User Journey

```bash
# 1. User already has NATS running (Omni, their own apps, whatever)
# 2. User has notif CLI installed with API key

# Install the bridge as a system service
notif connect install --nats nats://localhost:4222 --cloud

# Start it
notif connect start

# Check status (human-readable by default)
notif connect status
# → Bridge: running (uptime: 2h15m)
# → Local NATS: connected (nats://localhost:4222)
# → Cloud: connected (leaf node to notif.sh)
# → Interceptors: 2 active
# → Throughput: 142 msgs/s
# → Last heartbeat: 8s ago

# Machine-readable output
notif connect status --json

# View logs
notif connect logs

# Stop / remove
notif connect stop
notif connect uninstall

# Run in foreground (dev/debug)
notif connect run --nats nats://localhost:4222 --cloud
```

## Architecture

```
┌──────────────────────────────────────────────────┐
│  notif connect (system service)                  │
│                                                  │
│  ┌──────────────┐    ┌─────────────────────┐     │
│  │  Interceptor  │    │  Leaf Node Config   │     │
│  │  Manager      │    │  (NKey auth to      │     │
│  │  (jq + map)   │    │   cloud account)    │     │
│  └──────┬───────┘    └──────────┬──────────┘     │
│         │                       │                │
│         ▼                       ▼                │
│  ┌──────────────────────────────────────────┐    │
│  │  Local NATS connection                   │    │
│  │  (user's NATS with leaf node to cloud)   │    │
│  └──────────────────────────────────────────┘    │
└──────────────────────────────────────────────────┘
         │                          │
         ▼                          ▼
  [Local NATS subjects]     [notif.sh cloud NATS]
  (Omni, apps, etc)         (via leaf node protocol)
```

### How leaf node works

The local NATS server is configured with a leaf node connection to notif.sh cloud:

```
# Auto-generated in local nats-server.conf by notif connect
leafnodes {
  remotes [
    {
      url: "nats-leaf://cloud.notif.sh:7422"
      credentials: "/path/to/account.creds"     # NKey for org's account
      account: "<ORG_ACCOUNT_PUBLIC_KEY>"
    }
  ]
}
```

Events flow bidirectionally at the NATS protocol level:
- **Outbound**: local publish → leaf node → cloud account streams
- **Inbound**: cloud publish → leaf node → local NATS subjects
- Zero application-level overhead — NATS handles everything

### What `notif connect` actually does

1. **Provisions credentials**: calls notif.sh API to get NKey credentials for the org's NATS account
2. **Configures leaf node**: writes leaf node config snippet for the user's existing NATS server
3. **Runs interceptors**: connects to local NATS, runs jq transforms on configured subjects
4. **Reports status**: writes heartbeat to status file, logs to file

The bridge does NOT proxy events through HTTP. The leaf node connection handles
all cloud sync. The bridge only manages interceptors and lifecycle.

## Implementation Plan

### 1. Service scaffold (`cmd/notif/connect.go`)

New cobra subcommand tree:

```
notif connect install [flags]   # register system service + configure leaf node
notif connect start             # start service
notif connect stop              # stop service
notif connect status            # show bridge status
notif connect logs              # tail service logs
notif connect uninstall         # remove system service + clean credentials
notif connect run               # run in foreground (for dev/debug)
```

Dependencies:
- `github.com/kardianos/service` — cross-platform service management
- `internal/interceptor` — existing interceptor code
- Phase 2 (`--cloud`) blocked by: `nats-accounts` (needs NATS account + NKey credentials)

### 2. Credential provisioning

On `notif connect install --cloud`:

1. CLI authenticates with NOTIF_API_KEY (existing)
2. Calls `POST /api/v1/connect/provision` → returns NKey seed **once**
3. CLI displays the NKey seed to the user and writes to `~/.notif/connect.creds`
4. Generates leaf node config snippet

**AWS IAM credential model**: The NKey seed is returned exactly once in the provision
response. It is never stored server-side and cannot be retrieved again. If lost, user
must re-provision (which revokes the old credential and issues a new one).

```go
type ProvisionResponse struct {
    NKeySeed     string `json:"nkey_seed"`      // shown ONCE, never again
    AccountPubKey string `json:"account_pub_key"`
    LeafNodeURL  string `json:"leaf_node_url"`
}
```

Security requirements:
- **TLS required**: provision endpoint rejects non-HTTPS requests
- **Audit logged**: every provision emits `credential.provision` audit event (who, when, IP)
- **Rate limited**: max 3 provisions per org per hour
- **Revoke on re-provision**: old credential revoked when new one is issued

CLI output on provision:
```
⚠ NKey seed shown ONCE. Save it now — it cannot be retrieved again.

  NKEY_SEED: SUAM...

Credentials written to ~/.notif/connect.creds
Leaf node config written to ~/.notif/leafnode.conf
```

### 3. Leaf node configuration

User must have their own NATS server running. `notif connect` patches the config:

```bash
notif connect install --nats nats://localhost:4222 --cloud \
  --nats-config /etc/nats/nats-server.conf
```
Appends leaf node block to existing config. Requires NATS reload (`nats-server --signal reload`).

No managed/embedded NATS mode. Users without NATS install it themselves.

### 4. Bridge runtime (`internal/bridge/bridge.go`)

```go
type Bridge struct {
    localNatsURL   string
    cloudEnabled   bool
    topics         []string
    icfgPath       string       // interceptors.yaml path

    nc             *nats.Conn
    js             jetstream.JetStream
    stream         jetstream.Stream
    interceptor    *interceptor.Manager  // nil if no config
    status         *StatusReporter
}

func (b *Bridge) Start(ctx context.Context) error
func (b *Bridge) Stop()
func (b *Bridge) Status() BridgeStatus
```

The Bridge connects to local NATS only. Cloud sync happens at the NATS protocol
level via leaf node — the Bridge doesn't need to know about it.

### 5. JetStream stream handling

Creates a `NOTIF_BRIDGE` stream on local NATS for interceptors:

```go
stream, _ := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
    Name:     "NOTIF_BRIDGE",
    Subjects: topics,              // from --topics
    Storage:  jetstream.FileStorage,
    MaxAge:   24 * time.Hour,
})
```

Interceptors consume from this stream. If `--cloud` is enabled, events also flow
to cloud via leaf node (NATS handles this automatically for matching subjects).

**Stream conflict detection**: On install, check if any existing JetStream stream
already captures the configured `--topics` subjects. If conflict detected:
- Offer `--stream <name>` to reuse the existing stream instead of creating NOTIF_BRIDGE
- Fail with clear error message explaining the conflict

```go
// On install, before creating stream:
for _, existing := range js.StreamNames() {
    info, _ := js.StreamInfo(existing)
    if subjectsOverlap(info.Config.Subjects, topics) {
        return fmt.Errorf("stream %q already captures subjects %v; use --stream %s to reuse it",
            existing, info.Config.Subjects, existing)
    }
}
```

Consumer config includes `MaxAckPending` for backpressure:

```go
consumer, _ := js.CreateOrUpdateConsumer(ctx, "NOTIF_BRIDGE", jetstream.ConsumerConfig{
    Durable:       "interceptor-" + name,
    FilterSubjects: []string{from},
    AckPolicy:     jetstream.AckExplicitPolicy,
    MaxAckPending: 1000,  // backpressure: max 1000 unacked messages
})
```

### 6. Config resolution

Priority order:
1. CLI flags (`--nats`, `--cloud`, `--topics`, `--interceptors`)
2. Environment variables (`NATS_URL`, `NOTIF_API_KEY`, `NOTIF_CLOUD_URL`)
3. Config file (`~/.notif/connect.yaml`) — persisted by `install`

On `notif connect install`, flags are persisted to `~/.notif/connect.yaml` so the
service knows its config on boot without flags.

```yaml
# ~/.notif/connect.yaml (auto-generated by install)
nats_url: nats://localhost:4222
cloud: true
cloud_leaf_url: nats-leaf://cloud.notif.sh:7422
credentials: ~/.notif/connect.creds
topics:
  - "orders.>"
  - "alerts.>"
interceptors: /path/to/interceptors.yaml
```

### 7. Status reporting

`notif connect status` defaults to human-readable output. Use `--json` for machine-readable.

**Human-readable (default):**
```
Bridge:        running (uptime: 2h15m)
Local NATS:    connected (nats://localhost:4222)
Cloud:         connected (leaf node to notif.sh, account: org_A)
Interceptors:  2 active (14,523 processed, 0 errors)
Throughput:    142 msgs/s (47.2 KB/s)
Heartbeat:     8s ago
```

**JSON (`--json` flag):**
```json
{
  "state": "running",
  "pid": 12345,
  "last_heartbeat": "2026-02-17T22:30:05Z",
  "nats": {
    "url": "nats://localhost:4222",
    "connected": true
  },
  "cloud": {
    "leaf_node": true,
    "connected": true,
    "account": "org_A"
  },
  "interceptors": {
    "active": 2,
    "processed": 14523,
    "errors": 0
  },
  "throughput": {
    "msgs_per_sec": 142,
    "bytes_per_sec": 48320
  },
  "uptime": "2h15m"
}
```

**Heartbeat & staleness detection:**
- Heartbeat written every **10 seconds** by the running service
- `now() - last_heartbeat > 30s` → status shows "stale (no heartbeat for 30s)"
- `now() - last_heartbeat > 60s` → status shows "dead (no heartbeat for 60s)"

**Config drift detection:** On each heartbeat, bridge compares running config
against `~/.notif/connect.yaml`. If mismatch detected (e.g., user edited config
while service running), status shows warning:
```
⚠ Config drift detected: nats_url changed (running: localhost:4222, config: localhost:4223)
  Run `notif connect restart` to apply.
```

### 8. Logging

Service logs to `~/.notif/connect.log`. `notif connect logs` tails it.
Uses `slog` with JSON format (same as notifd).

**Log rotation** via `lumberjack`:
```go
logger := &lumberjack.Logger{
    Filename:   "~/.notif/connect.log",
    MaxSize:    50,    // MB
    MaxBackups: 3,
    MaxAge:     14,    // days
    Compress:   true,
}
```

## File Structure

```
cmd/notif/
  connect.go              # cobra subcommands (install/start/stop/status/logs/run)

internal/bridge/
  bridge.go               # Bridge struct (wires interceptor + leaf node lifecycle)
  config.go               # Config resolution (flags → env → file) + drift detection
  status.go               # Status reporting (heartbeat 10s, stale 30s, dead 60s)
  provision.go            # Credential provisioning (AWS IAM model, show-once)
  leafnode.go             # Leaf node config generation (patch mode only)

# Reused as-is:
internal/interceptor/      # existing
```

## What this enables

```
Today:
  notif emit "topic" '{"data": ...}'       # cloud-only
  notif subscribe "topic.>"                 # cloud-only

With notif connect:
  [Local NATS app] → publish to local NATS
       ↓
  notif connect (interceptors on local)
       ↓
  Leaf node → events appear in notif.sh cloud
       ↓
  notif.sh webhooks fire, dashboard shows events

  Also:
  notif emit "topic" '{"data": ...}'       # cloud
       ↓
  Leaf node → event appears on local NATS
       ↓
  Local apps receive it natively
```

## What was removed

The original design used `internal/federation/client.go` (HTTP/WS bridge) for cloud
sync. This has been **replaced entirely by NATS leaf node protocol**:

| | HTTP/WS Bridge (removed) | Leaf Node (current) |
|---|---|---|
| Protocol | Application-level (HTTP + WebSocket) | NATS protocol-level |
| Overhead | JSON serialize → HTTP POST → deserialize | Zero-copy NATS routing |
| Auth | Bearer token per request | NKey credentials, one-time setup |
| Reliability | HTTP retry logic, reconnect loops | NATS built-in reconnect |
| Bidirectional | Requires separate inbound WS + outbound HTTP | Automatic, both directions |
| Code | ~200 lines in federation/client.go | Config-only (leaf node block) |

If future clients need HTTP/WS bridge (e.g., corporate firewall blocking
non-443 ports), it can be reimplemented as a separate mode. Not needed now.

## Acceptance Criteria

### Phase 1 (local-only)
- [ ] `notif connect install --nats nats://localhost:4222` registers system service
- [ ] `notif connect start` → service runs, connects to local NATS
- [ ] `notif connect status` → human-readable output (default), `--json` for machine-readable
- [ ] Heartbeat: 10s interval, stale at 30s, dead at 60s
- [ ] Config drift detection: warns if running config differs from `~/.notif/connect.yaml`
- [ ] Interceptors from `--interceptors` YAML work with `MaxAckPending: 1000` backpressure
- [ ] `notif connect stop && notif connect uninstall` → clean removal
- [ ] Works on macOS (launchd) and Linux (systemd)
- [ ] Survives reboot (service auto-starts)
- [ ] `notif connect run` works for foreground dev/debug
- [ ] Stream conflict detection: warns if `--topics` subjects already captured by existing stream
- [ ] Log rotation via lumberjack (50MB, 3 backups, 14 days)

### Phase 2 (after nats-accounts)
- [ ] `--cloud` flag provisions NKey credentials via `POST /api/v1/connect/provision`
- [ ] AWS IAM credential model: NKey seed shown once, TLS required, audit logged
- [ ] Leaf node connects local NATS to cloud account (NATS protocol, not HTTP)
- [ ] Events flow bidirectionally via leaf node (local publish → cloud, cloud emit → local)
- [ ] `notif connect uninstall` cleans credentials and leaf node config
