# notifd-cloud-bridge

## Problem
Any dev with a local NATS server wants to connect it to notif.sh — get interceptors,
transforms, and optionally sync events to the managed cloud. Zero external deps
(no Postgres). Works as a subcommand of the existing `notif` CLI.

## Scope

**IN:**
- `notif connect` subcommand with daemon lifecycle (install/start/stop/status/logs/uninstall)
- Connects to any local NATS (via `--nats` flag or `NATS_URL` env)
- Interceptors (jq transform, subject mapping) from YAML config
- Federation bridge to notif.sh cloud (via `--cloud` flag, uses existing `NOTIF_API_KEY`)
- Cross-platform daemon via `kardianos/service` (systemd/launchd/Windows Service)
- Topic filtering (`--topics "orders.>,alerts.>"`)

**OUT (for now):**
- Local HTTP API / webhooks / scheduling
- Postgres or any external DB
- Schema validation across boundaries
- Dashboard local
- Windows support (stretch goal)

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Where does it live? | `notif connect` subcommand | CLI already installed, already has auth. No new binary. |
| Process management | `kardianos/service` | Single Go API → systemd (Linux) / launchd (Mac) / Windows Service. Auto-restart, survives reboot. |
| Storage | None (NATS JetStream only) | Interceptor + federation use durable consumers. Zero external deps. |
| Cloud connection | Opt-in via `--cloud` flag | Default is local-only. Cloud uses existing federation client (WS sub + HTTP emit). |
| Config | Env vars + optional YAML | `NATS_URL` + `NOTIF_API_KEY` for basics. `--interceptors` for transforms. |

## Risks

| Risk | Mitigation |
|---|---|
| `kardianos/service` maturity | Battle-tested (Grafana Agent, Telegraf). 4k+ stars. |
| First long-running process in `notif` CLI | Isolated in `connect` subcommand. Rest of CLI unchanged. |
| Cloud API changes break federation | Federation client already abstracts protocol. Version the WS handshake. |
| User's NATS has different JetStream config | `notif connect` creates its own consumers. Doesn't touch user's streams — connects to existing ones. |

## Acceptance Criteria

1. `notif connect install && notif connect start` with a local NATS → interceptors processing
2. `--cloud` flag → events appear in notif.sh cloud dashboard
3. `notif connect status` shows bridges active + messages/s throughput
4. Works on macOS and Linux (Windows is stretch goal)

## CLI Interface

```bash
# Install as system service
notif connect install \
  --nats nats://localhost:4222 \
  --topics "orders.>,alerts.>" \
  --interceptors ./interceptors.yaml \
  --cloud  # optional: sync to notif.sh

# Lifecycle
notif connect start
notif connect stop
notif connect status
notif connect logs
notif connect uninstall
```

## Architecture

```
Local-only mode:
  [Any NATS] → notif connect → interceptors (jq transform)
                    │
                    └→ republish to mapped subjects

Hybrid mode (--cloud):
  [Any NATS] → notif connect → interceptors
                    │
                    ├→ republish locally
                    └→ federation client (WS+HTTP) → notif.sh cloud
```

## Component Reuse

| Existing code | How it's used in `notif connect` |
|---|---|
| `internal/interceptor/` | Imported directly. Same manager, same config. |
| `internal/federation/` | Imported directly. Cloud bridge = federation outbound/inbound. |
| `internal/federation/client.go` | WS subscribe + HTTP emit to cloud. |
| `NOTIF_API_KEY` | Already in user's env from `notif auth`. Reused for cloud federation. |

## What's NOT needed from notifd

- `internal/server/` (HTTP API)
- `internal/config/` → DatabaseURL, ClerkSecretKey, CORSOrigins
- `pgxpool` dependency
- The entire Postgres layer
