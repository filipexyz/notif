# Federation & Cloud Platform Design

> Status: **Design** | Date: 2026-02-17

## Current State (v0 — shipped)

Three NATS-native primitives for connecting notif instances to external systems:

```
internal/interceptor/   ~190 lines   Subscribe -> jq transform -> republish
internal/federation/    ~320 lines   WS subscribe + HTTP emit (notif-to-notif)
nats-server.conf        config-only  Leafnode listener (commented, opt-in)
```

All opt-in via env vars (`INTERCEPTORS_CONFIG`, `FEDERATION_CONFIG`) or uncommenting
the leafnode block. Zero impact on existing cloud deployment.

### What each primitive does

**Interceptor** — Durable JetStream consumer that subscribes to a subject pattern,
optionally transforms the payload via jq, and republishes to a different subject.
Chain-based loop prevention via `X-Notif-Interceptor` header. Config validated at
startup (empty fields, commas in names, duplicates). Rollback on partial start failure.

**Federation Client** — Go-native notif client. Inbound bridges subscribe via
WebSocket and republish to local JetStream. Outbound bridges consume from local
JetStream and emit via HTTP POST. Durable consumers, exponential backoff reconnect,
4xx early-return on emit.

**Leafnode** — NATS server-level config connecting two NATS servers bidirectionally.
Zero code. Subjects flow transparently. Supports subject mapping at the server level.

## Target Architecture

```
                     notif.sh cloud
 ┌──────────────────────────────────────────────────────────┐
 │                                                          │
 │  ┌───────────┐   ┌───────────┐   ┌───────────┐          │
 │  │ acme-corp │   │partner-inc│   │ startup-x │          │
 │  │ (Account) │   │ (Account) │   │ (Account) │          │
 │  └─────┬─────┘   └─────┬─────┘   └─────┬─────┘          │
 │        │               │               │                │
 │        ▼               ▼               ▼                │
 │  ┌──────────────────────────────────────────────────┐    │
 │  │         NATS Supercluster (managed)              │    │
 │  │   Account isolation + JetStream per tenant       │    │
 │  │   export/import for cross-account federation     │    │
 │  └──────────────────────┬───────────────────────────┘    │
 │                         │                                │
 └─────────────────────────┼────────────────────────────────┘
                           │ leaf nodes
              ┌────────────┼────────────┐
              ▼                         ▼
 ┌────────────────────┐    ┌────────────────────┐
 │  Self-hosted notif │    │  Self-hosted notif │
 │  (on-prem / edge)  │    │  (another client)  │
 │  leaf node to cloud│    │  leaf node to cloud│
 └────────────────────┘    └────────────────────┘
```

### Core principle

**1 NATS Account = 1 notif tenant/org.** This gives us:

- **Isolation** — Subjects, streams, KV stores are invisible across accounts
- **Federation** — Subject export/import between accounts (controlled)
- **Billing** — `sum(msgs)` per account, no external metering needed
- **Auth** — Account JWTs, abstracted behind `notif auth`

## The `notif://` Protocol

```
notif://acme-corp/orders.created
  │        │          │
  │        │          └── subject (topic)
  │        └── namespace (instance/org = NATS Account)
  └── protocol (resolves to NATS connection)
```

### Resolution

| Context | How `notif://acme-corp/orders.created` resolves |
|---|---|
| Both on cloud | Subject export/import between Accounts (microsecond latency) |
| Cloud + self-hosted | Leaf node relay through cloud hub |
| Self-hosted only | Manual config (`notif federation add`) or direct leaf node |

### What `notif://` solves

| Problem | Solution |
|---|---|
| Discovery | Cloud registry (like DNS). Self-hosted: manual config |
| Auth | NATS Account JWTs, abstracted behind `notif auth` |
| Namespace isolation | Subject mapping per account (automatic) |
| Replay | JetStream `DeliverByStartTime`, gated by replay policy |
| Metering | Per-account stream metrics (native JetStream) |

## Product Tiers

```
┌─────────────┬─────────────────────────────────────────────────┐
│ Free / Dev  │ 1 account on cloud, low limits                  │
│             │ emit/subscribe works directly                    │
│             │ No federation                                    │
├─────────────┼─────────────────────────────────────────────────┤
│ Pro         │ Account on cloud, higher limits                  │
│             │ Webhooks, scheduling, schemas                    │
│             │ Federation with other Pro tenants                │
├─────────────┼─────────────────────────────────────────────────┤
│ Self-hosted │ Everything runs local (OSS, MIT)                 │
│             │ No cloud, no cross-org federation                │
│             │ (unless manually configured)                     │
├─────────────┼─────────────────────────────────────────────────┤
│ Hybrid      │ Self-hosted + leaf node to cloud                 │
│             │ Local events stay local                          │
│             │ Exported events flow to cloud                    │
│             │ Federation via cloud relay                       │
├─────────────┼─────────────────────────────────────────────────┤
│ Enterprise  │ Dedicated supercluster or region                 │
│             │ SLA, audit log, compliance                       │
│             │ Schema validation across boundaries              │
│             │ Unified multi-org dashboard                      │
└─────────────┴─────────────────────────────────────────────────┘
```

## Billing Model

```
Evento interno (same account)      → conta pro tenant
Evento federado (cross-account)    → publisher paga emit, subscriber paga receive
Evento cloud relay (self→cloud→self) → relay fee
```

JetStream already has per-stream/consumer metrics. Since each account owns its
own streams, billing = `sum(msgs)` per account. No external metering system needed.

## Roadmap

### Phase 1: CLI Sugar (highest ROI, lowest effort)

> Prerequisite: v0 shipped (done)

**Goal:** Replace manual YAML editing with one-liner CLI commands.

```bash
# Add a federation partner
notif federation add partner-inc \
  --endpoint wss://partner.notif.sh \
  --api-key nsh_xxx \
  --direction both

# Export a topic
notif federation export orders.created --to partner-inc

# Import from a partner
notif federation import orders.created \
  --from partner-inc \
  --as external.partner.orders.created

# List active federations
notif federation ls

# Remove
notif federation rm partner-inc
```

**Implementation:**

- CLI commands generate/update `federation.yaml` and `interceptors.yaml`
- Hot-reload via SIGHUP or NATS signal (no restart needed)
- Validation at CLI level (endpoint reachable, API key valid)
- Stored in `federation.yaml` alongside `nats-server.conf`

**Acceptance criteria:**

- [ ] `notif federation add` creates bridge config + interceptor config
- [ ] `notif federation export/import` creates subject mappings
- [ ] `notif federation ls` shows active bridges with status
- [ ] `notif federation rm` cleans up config + stops bridge
- [ ] Hot-reload works without daemon restart
- [ ] Manual YAML still works (CLI is sugar, not replacement)

### Phase 2: Replay Policy

> Prerequisite: Phase 1

**Goal:** Allow federated partners to replay historical events within authorized
time windows.

```bash
# Grant partner replay access (last 24h of orders.created)
notif federation grant partner-inc \
  --subject orders.created \
  --replay 24h

# Partner requests replay
notif federation replay notif://acme-corp/orders.created \
  --since 2h
```

**Implementation:**

- Replay policy stored per federation link (who can replay what, how far back)
- Interceptor becomes the enforcement point — checks policy before allowing replay
- JetStream `DeliverByStartTime` for the actual replay mechanism
- Audit log entry for every replay request (who, what, when, how much)

**Key decisions:**

- Replay creates a temporary consumer with `DeliverByStartTimePolicy`
- Rate-limited (e.g., 1 replay request per minute per subject)
- Events delivered to the existing federation bridge (no new connection)

### Phase 3: NATS Accounts (multi-tenancy)

> Prerequisite: Phase 2 | **This is the riskiest step**

**Goal:** Each notif org = 1 NATS Account. Full isolation. Foundation for cloud.

```
Operator (notif.sh)
  └── Account: acme-corp
  │     └── User: api-key-1 (pub: events.acme-corp.>)
  │     └── User: api-key-2 (sub: events.acme-corp.>)
  │     └── Export: orders.created → partner-inc
  │
  └── Account: partner-inc
        └── User: api-key-3
        └── Import: orders.created from acme-corp
```

**Implementation:**

- Use `nsc` internally to manage operator/account/user JWTs
- **Completely abstract the JWT layer** — users never see JWTs
- `notif auth` generates credentials that map to Account users
- Existing API keys become Account user credentials
- Migration: current single-account setup → multi-account with backward compat

**What changes in existing code:**

- `internal/nats/client.go` — Connect with Account credentials instead of plain URL
- `internal/config/config.go` — Account ID becomes part of connection config
- `cmd/notifd/main.go` — Operator setup on first boot
- Federation — Becomes export/import between Accounts (replaces WS+HTTP client
  for cloud-to-cloud; federation client survives for self-hosted↔self-hosted
  without cloud)

**Key decisions:**

- The federation client (WS+HTTP) becomes the **fallback** for cases without
  shared NATS infrastructure. When both parties are on the same NATS cluster
  (cloud), federation is native export/import (microsecond latency, no HTTP)
- Interceptor remains relevant — it handles transform/filter regardless of
  transport mechanism
- Leafnode remains relevant — it's how self-hosted connects to cloud

**Migration strategy:**

1. Deploy with single operator, single system account (backward compat)
2. New orgs get their own Account
3. Existing orgs migrated gradually (feature flag)
4. Once all orgs are on Accounts, remove legacy single-account code

### Phase 4: Cloud Hosted

> Prerequisite: Phase 3

**Goal:** notif.sh cloud as managed NATS supercluster with signup → account →
credentials flow.

```bash
# Sign up (creates Account)
notif auth signup --email luis@acme.com

# Get credentials
notif auth login
# → Downloads account credentials, configures local CLI

# Everything works
notif emit orders.created '{"id": 123}'
notif subscribe "orders.>"
```

**Implementation:**

- Cloud runs NATS supercluster (managed)
- Signup API creates NATS Account + notif org
- `notif auth` generates Account user JWTs
- Self-hosted can connect as leaf node: `notif cloud connect`
- Cloud becomes the discovery registry for `notif://`

**Infrastructure:**

```
Cloud:
  - NATS supercluster (3+ nodes, multi-region)
  - Operator JWT (managed by notif.sh)
  - Account provisioning API
  - Billing metering (reads JetStream account metrics)

Self-hosted connecting to cloud:
  - nats-server.conf with leaf node pointing to cloud
  - Account credentials from `notif auth`
```

### Phase 5: `notif://` Global Resolution

> Prerequisite: Phase 4

**Goal:** `notif://acme-corp/orders.created` resolves globally via cloud registry.

```bash
# On cloud — just works
notif subscribe "notif://partner-inc/orders.created"

# Self-hosted connected to cloud — also works
notif subscribe "notif://partner-inc/orders.created"

# Self-hosted NOT on cloud — manual resolution
notif federation add partner-inc --endpoint nats-leaf://partner:7422
notif subscribe "notif://partner-inc/orders.created"
```

**Implementation:**

- Cloud maintains registry: account name → NATS routing info
- `notif://` URI parsed at CLI and SDK level
- Resolution:
  1. Check if target account is on same cluster → direct export/import
  2. Check if target is on cloud → route via gateway
  3. Check if target is in local federation config → use configured endpoint
  4. Fail with "unknown org, use `notif federation add`"

### Phase 6: Platform Features (future)

- Schema validation across federation boundaries
- Unified dashboard (events flowing between instances)
- Marketplace of integrations (pre-built interceptors)
- Audit log for cross-boundary events
- Rate limiting per federation link
- HMAC signatures on federated events

## The Flywheel

```
More tenants on cloud
        ↓
More federation possibilities (network effect)
        ↓
notif:// becomes more valuable (more orgs reachable)
        ↓
Self-hosted wants to connect to cloud (hybrid)
        ↓
More tenants on cloud
```

Same model as Cloudflare Tunnel / Tailscale — software is open source and runs
locally, but the cloud is the "control plane" that connects everything. The value
isn't in the software, it's in the network.

## Component Lifecycle

How each v0 primitive evolves through the roadmap:

| Component | Phase 1-2 | Phase 3+ (Accounts) | Phase 4+ (Cloud) |
|---|---|---|---|
| **Interceptor** | Transform + filter engine | + replay enforcement | + schema validation |
| **Federation Client** | Primary transport (WS+HTTP) | Fallback for no-cloud | Deprecated for cloud-to-cloud |
| **Leafnode** | Manual config | Self-hosted → cloud transport | Default for hybrid tier |
| **YAML config** | CLI-generated | Migrates to Account-based config | Managed by cloud API |

## Files Reference

```
# Current (v0)
internal/interceptor/interceptor.go    # Core intercept loop
internal/interceptor/config.go         # YAML config + validation
internal/interceptor/manager.go        # Multi-interceptor lifecycle
internal/federation/federation.go      # Bridge manager + config
internal/federation/client.go          # WS subscribe + HTTP emit
internal/nats/client.go                # NATS connection wrapper
cmd/notifd/main.go                     # Wiring + shutdown order
nats-server.conf                       # NATS server config (leafnode ready)

# Example configs
interceptors.example.yaml
federation.example.yaml
leafnode.example.conf

# Tests (24 total)
internal/interceptor/interceptor_test.go   # 12 tests
internal/federation/federation_test.go     # 8 tests
internal/nats/leafnode_test.go             # 4 tests
```
