# Design: NATS Accounts (Account-per-Tenant)

> Protocol-level tenant isolation for notif.sh cloud via NATS Accounts.

## Problem

notif.sh cloud hoje usa um único NATS account com isolamento a nível de aplicação
(subjects prefixados com `org_id.project_id` no código Go). Isso impede federação
real entre tenants, não oferece isolamento de protocolo, e não permite billing
granular por uso de recursos NATS.

## Current State

```
┌──────────────────────────────────────────────────┐
│  NATS Server (single account)                    │
│                                                  │
│  Stream: NOTIF_EVENTS    (events.>)              │
│  Stream: NOTIF_DLQ       (dlq.>)                 │
│  Stream: NOTIF_WEBHOOK_RETRY (webhook-retry.>)   │
│                                                  │
│  events.org_A.proj_1.orders.>   ← Tenant A      │
│  events.org_B.proj_1.orders.>   ← Tenant B      │
│                                                  │
│  Isolation: application-level (Go code checks    │
│  org_id in subject before allowing operations)   │
│                                                  │
│  notifd: single *nats.Conn, single stream handle │
└──────────────────────────────────────────────────┘
```

Problems:
1. **No protocol isolation** — a bug in Go code could leak Tenant B's events to Tenant A
2. **No federation** — can't export/import subjects between accounts (there's only one)
3. **No granular billing** — can't measure per-tenant NATS resource usage natively
4. **No leaf node scoping** — self-hosted NATS can't connect with account-scoped credentials
5. **Noisy neighbor** — one tenant with high volume affects the shared stream for all

## Target State

```
┌──────────────────────────────────────────────────────────┐
│  NATS Server (multi-account via nats-resolver full)      │
│                                                          │
│  ┌─ Operator: notif.sh ─────────────────────────────┐    │
│  │                                                   │    │
│  │  ┌─ System Account (notifd) ──────────────────┐   │    │
│  │  │  $SYS.> monitoring only                     │   │    │
│  │  │  Per-account connections via ClientPool      │   │    │
│  │  └────────────────────────────────────────────┘   │    │
│  │                                                   │    │
│  │  ┌─ Account: org_A ──────────────────────────┐    │    │
│  │  │  Stream: NOTIF_EVENTS_org_A  (events.>)    │    │    │
│  │  │  Stream: NOTIF_DLQ_org_A     (dlq.>)       │    │    │
│  │  │  Stream: NOTIF_WEBHOOK_RETRY_org_A          │    │    │
│  │  │  Limits: 100 conns, 1GB data               │    │    │
│  │  └────────────────────────────────────────────┘    │    │
│  │                                                   │    │
│  │  ┌─ Account: org_B ──────────────────────────┐    │    │
│  │  │  Stream: NOTIF_EVENTS_org_B  (events.>)    │    │    │
│  │  │  Stream: NOTIF_DLQ_org_B     (dlq.>)       │    │    │
│  │  │  Stream: NOTIF_WEBHOOK_RETRY_org_B          │    │    │
│  │  │  Limits: 50 conns, 500MB data              │    │    │
│  │  └────────────────────────────────────────────┘    │    │
│  │                                                   │    │
│  └───────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────┘
```

## Architecture

### JWT Model

```
Operator (notif.sh)
  ├── System Account (notifd control plane)
  │   └── User: notifd-system (NKey auth, $SYS.> + claims management)
  │
  ├── Account: org_A
  │   ├── User: notifd-org_A (NKey — notifd's per-account connection)
  │   ├── User: api-user (for notif CLI / SDK connections)
  │   ├── User: leafnode-user (for self-hosted NATS via notif connect)
  │   └── Signing Key: SK-org_A (for user JWT issuance)
  │
  └── Account: org_B
      ├── User: notifd-org_B (NKey — notifd's per-account connection)
      ├── User: api-user
      └── Signing Key: SK-org_B
```

### Resolver: nats-resolver full

```
Go code generates JWT → publish to $SYS.REQ.CLAIMS.UPDATE → hot reload (no restart)
```

Why nats-resolver full:
- **Dynamic**: add/remove accounts without NATS restart
- **Self-contained**: JWTs stored in NATS JetStream (replicated)
- **Programmatic push**: via `$SYS.REQ.CLAIMS.UPDATE` from system connection
- **Backup**: JWTs recoverable from NATS

Why NOT alternatives:
- `memory` resolver: static, requires server restart on account changes
- `url` resolver: legacy, polling-based, external HTTP dependency
- `nsc` CLI subprocess: fragile, needs persistent volume for `~/.nsc/`, concurrency issues

### JWT Generation: Programmatic (no nsc)

Instead of shelling out to `nsc`, import the underlying Go libraries directly:

```go
import (
    "github.com/nats-io/nkeys"     // Key generation (NKey pairs)
    "github.com/nats-io/jwt/v2"    // JWT creation and signing
)
```

Account lifecycle via Go code:
1. Generate NKey pair for new account: `nkeys.CreateAccount()`
2. Create account JWT: `jwt.NewAccountClaims(publicKey)`
3. Set limits: `claims.Limits.Conn = 100`, `claims.Limits.Data = 1GB`
4. Sign with operator key: `claims.Encode(operatorKeyPair)`
5. Push to NATS: publish signed JWT to `$SYS.REQ.CLAIMS.UPDATE`
6. NATS hot-reloads — account active immediately

Benefits over nsc subprocess:
- **Single binary**: no external CLI dependency
- **No persistent volume**: no `~/.nsc/` directory to manage
- **Concurrency safe**: Go mutexes, not filesystem locks
- **Testable**: unit test JWT generation without subprocess mocking
- **Atomic**: JWT generation + push in one Go function

Operator key pair stored as env var (`OPERATOR_SEED`) or file mount.

### Audit Log (Prerequisite)

Every security-sensitive operation is logged to a structured audit trail. This is a
prerequisite for all other designs — no JWT push, credential provisioning, or federation
operation happens without an audit record.

```sql
CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    timestamp   TIMESTAMPTZ DEFAULT now(),
    actor       TEXT NOT NULL,               -- "notifd", "api:user@org_a", "cli:admin"
    action      TEXT NOT NULL,               -- "account.create", "jwt.push", "credential.provision"
    org_id      VARCHAR(32) REFERENCES orgs(id),
    target      TEXT,                        -- what was acted on (account ID, export ID, etc.)
    detail      JSONB,                       -- action-specific metadata
    ip_address  INET                         -- client IP for API/CLI actions
);

CREATE INDEX idx_audit_log_org_id ON audit_log(org_id);
CREATE INDEX idx_audit_log_action ON audit_log(action);
CREATE INDEX idx_audit_log_timestamp ON audit_log(timestamp);
```

Actions logged:
- `account.create` / `account.delete` — org lifecycle
- `jwt.push` — every RebuildAndPush with before/after claim diff
- `credential.provision` — NKey seed emitted (who, when, IP)
- `credential.rotate` — user NKey regenerated on restart
- `federation.export.create` / `federation.import.create` / `federation.rm`
- `tier.change` — billing tier upgrade/downgrade

CLI: `notif audit [--org <id>] [--action <action>] [--since <duration>]`

Also emitted as structured slog events for external log aggregation.

### OPERATOR_SEED Rotation Runbook

OPERATOR_SEED is the root of trust. It signs all account JWTs. Rotation procedure:

```
1. Generate new operator NKey pair:
   nkeys -gen operator → new OPERATOR_SEED + OPERATOR_PUB

2. Update NATS server config:
   Replace operator public key in nats-server.conf trusted_keys

3. Rebuild ALL account JWTs with new operator key:
   notif accounts rebuild-all --operator-seed <new_seed>
   (Iterates orgs table, RebuildAndPush each account JWT)

4. Rotate env var / secret mount:
   Update OPERATOR_SEED in deployment config

5. Restart notifd (picks up new OPERATOR_SEED)

6. Verify: notif accounts verify-all
   (Checks every account JWT was signed by current operator)
```

Frequency: annually or on suspected compromise. Audit logged as `operator.rotate`.

### JWT as Derived View (never stored)

JWTs are **never stored**. They are always rebuilt from DB state:

```
DB state (source of truth)          →  JWT (derived artifact)
─────────────────────────────       ─────────────────────────
orgs.billing_tier                   →  claims.Limits (conn, data, msg size)
orgs.nats_public_key                →  claims.Subject (account identity)
federation_exports (for this org)   →  claims.Exports[]
federation_imports (for this org)   →  claims.Imports[]
```

Every time a JWT needs updating (account create, limit change, federation
add/remove), the flow is:

```go
func RebuildAndPushAccountJWT(db *sql.DB, orgID string, operatorKP nkeys.KeyPair, sysConn *nats.Conn) error {
    org, _ := db.GetOrg(orgID)
    exports, _ := db.ListFederationExports(orgID)
    imports, _ := db.ListFederationImports(orgID)

    claims := jwt.NewAccountClaims(org.NatsPublicKey)
    claims.Name = org.Name
    applyLimitsFromTier(claims, org.BillingTier)
    applyExports(claims, exports)
    applyImports(claims, imports)

    signed, _ := claims.Encode(operatorKP)
    _, err := sysConn.Request("$SYS.REQ.CLAIMS.UPDATE", []byte(signed), 5*time.Second)

    // Audit log every push
    auditLog(db, "notifd", "jwt.push", orgID, map[string]any{
        "limits": claims.Limits,
        "exports": len(claims.Exports),
        "imports": len(claims.Imports),
    })
    return err
}
```

This means:
- **DB is single source of truth** — JWT is a materialized view
- **No JWT↔NATS desync** — JWT always reflects current DB state
- **No "load → modify → push" pattern** — always "read DB → build → push"
- **Idempotent** — calling RebuildAndPush twice produces the same JWT

### Transactional JWT Push

When an operation affects multiple accounts (e.g., federation activation touches
both org_a and org_b JWTs), both pushes must succeed or neither takes effect:

```go
func RebuildAndPushMultipleAccounts(db *sql.DB, orgIDs []string, operatorKP nkeys.KeyPair, sysConn *nats.Conn) error {
    // Phase 1: Build all JWTs (no side effects)
    jwts := make(map[string]string)
    for _, orgID := range orgIDs {
        signed, err := buildAccountJWT(db, orgID, operatorKP)
        if err != nil {
            return fmt.Errorf("build JWT for %s: %w", orgID, err)
        }
        jwts[orgID] = signed
    }

    // Phase 2: Push all JWTs (if any fails, log but don't leave partial state)
    var pushed []string
    for orgID, signed := range jwts {
        _, err := sysConn.Request("$SYS.REQ.CLAIMS.UPDATE", []byte(signed), 5*time.Second)
        if err != nil {
            // Rollback: rebuild and push already-pushed accounts without the new change
            slog.Error("JWT push failed, rolling back", "org", orgID, "pushed", pushed)
            rollbackJWTPush(db, pushed, operatorKP, sysConn)
            return fmt.Errorf("push JWT for %s: %w (rolled back %v)", orgID, err, pushed)
        }
        pushed = append(pushed, orgID)
    }
    return nil
}
```

Single-account operations (tier change, account create) use plain `RebuildAndPushAccountJWT`.
Multi-account operations (federation activate/revoke) use `RebuildAndPushMultipleAccounts`.

### NKey Credential Strategy for ClientPool

Each org's `notifd-{orgID}` user needs NKey credentials. Strategy: **regenerate on restart**.

On notifd boot or new org creation:
1. Generate fresh NKey pair: `nkeys.CreateUser()`
2. Create user JWT: `jwt.NewUserClaims(publicKey)`, set `IssuerAccount = orgAccountPubKey`
3. Sign with account signing key
4. Connect to NATS with the NKey pair

NKey pairs live **in memory only**. On notifd restart:
1. For each org in DB → generate new NKey pair → create new user JWT → reconnect
2. Old user JWTs expire naturally (short TTL) or are superseded

Why this works:
- **notifd controls user provisioning** — it can always create new users
- **No persistent credential storage** — no encrypted columns, no secrets manager
- **No credential leak risk** — NKey seeds never written to disk
- **Restart cost is minimal** — generating NKey + JWT + connect takes milliseconds per org

The only persistent secret is `OPERATOR_SEED` (env var or file mount), which is
needed to sign account JWTs. Everything else is derived at runtime.

### Connection Model: ClientPool

notifd does NOT use a god-connection that sees everything. Instead:

```go
// internal/nats/pool.go
type ClientPool struct {
    system  *nats.Conn              // $SYS only — monitoring, claims management
    clients map[string]*OrgClient   // per-account connections
    mu      sync.RWMutex
}

type OrgClient struct {
    orgID   string
    conn    *nats.Conn
    js      jetstream.JetStream
    streams OrgStreams               // NOTIF_EVENTS_{orgID}, DLQ, WEBHOOK_RETRY
}

func (p *ClientPool) Get(orgID string) (*OrgClient, error)
func (p *ClientPool) Add(orgID string, creds NKeyCredentials) error
func (p *ClientPool) Remove(orgID string) error
```

Request flow:
1. HTTP request arrives with API key
2. Resolve orgID from API key (existing auth middleware)
3. `pool.Get(orgID)` → per-account NATS connection
4. Publish to `events.{org_id}.{project_id}.{topic}` within that account's namespace
5. Each account has its own streams, consumers, subjects — fully isolated

System connection (`pool.system`) used only for:
- `$SYS.>` monitoring (metrics dashboard)
- `$SYS.REQ.CLAIMS.UPDATE` (account management)
- Never for event processing

### Stream Topology: Per-Account

Each org gets 3 streams, created on org signup:

```
Account: org_A
  NOTIF_EVENTS_org_A         subjects: ["events.>"]       24h, 1GB
  NOTIF_DLQ_org_A            subjects: ["dlq.>"]          7d
  NOTIF_WEBHOOK_RETRY_org_A  subjects: ["webhook-retry.>"] 24h
```

```go
// internal/nats/client.go — refactored
func EnsureStreamsForOrg(js jetstream.JetStream, orgID string) error {
    streams := []jetstream.StreamConfig{
        {Name: "NOTIF_EVENTS_" + orgID, Subjects: []string{"events.>"},
         Storage: jetstream.FileStorage, MaxAge: 24*time.Hour, MaxBytes: 1<<30},
        {Name: "NOTIF_DLQ_" + orgID, Subjects: []string{"dlq.>"},
         Storage: jetstream.FileStorage, MaxAge: 7*24*time.Hour},
        {Name: "NOTIF_WEBHOOK_RETRY_" + orgID, Subjects: []string{"webhook-retry.>"},
         Storage: jetstream.FileStorage, MaxAge: 24*time.Hour},
    }
    // CreateOrUpdate each stream
}
```

Stream limits map to billing tiers:
- Free: 256MB, 12h retention
- Pro: 1GB, 24h retention
- Enterprise: 10GB, 7d retention

### Orgs Table

Central entity for the entire system:

```sql
CREATE TABLE orgs (
    id              VARCHAR(32) PRIMARY KEY,        -- prj_ prefix or similar
    name            VARCHAR(255) NOT NULL,
    external_id     VARCHAR(255),                   -- Clerk org ID
    nats_public_key VARCHAR(128) NOT NULL UNIQUE,   -- NATS account public key
    billing_tier    VARCHAR(32) DEFAULT 'free',     -- free | pro | enterprise
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

-- Migrate existing org_id strings to reference this table
ALTER TABLE projects ADD CONSTRAINT fk_projects_org
    FOREIGN KEY (org_id) REFERENCES orgs(id);

ALTER TABLE api_keys ADD CONSTRAINT fk_api_keys_org
    FOREIGN KEY (org_id) REFERENCES orgs(id);
```

### Subject Model

Within each account, subjects follow the existing prefix pattern:

```
events.{org_id}.{project_id}.{topic}
```

Defense in depth: protocol-level isolation (NATS account boundary) +
application-level isolation (subject prefix validation in Go code). A bug
in one layer doesn't compromise the other.

### Leaf Node Strategy (Self-Hosted)

Self-hosted NATS (via `notif connect`) gets account-scoped NKey credentials:

```
Cloud NATS                          Self-Hosted NATS
┌──────────────┐                    ┌──────────────┐
│  Account:    │  ← leaf node ←    │  notif       │
│  org_A       │    (NKey auth)     │  connect     │
│              │  → leaf node →     │  (bridge)    │
└──────────────┘                    └──────────────┘
```

Leaf node sees ONLY subjects within org_A's account. Cannot access org_B.
No HTTP/WS bridge needed — NATS protocol handles everything.

### Boot Sequence

notifd startup must connect all org accounts before accepting HTTP traffic:

```go
func Boot(ctx context.Context, db *sql.DB, pool *ClientPool) error {
    // 1. Connect system account (required — fail fast if NATS unavailable)
    if err := pool.ConnectSystem(ctx); err != nil {
        return fmt.Errorf("NATS system connection failed: %w", err)
    }

    // 2. Load all orgs from DB
    orgs, _ := db.ListOrgs()

    // 3. Connect each org (parallel, with timeout)
    g, ctx := errgroup.WithContext(ctx)
    for _, org := range orgs {
        org := org
        g.Go(func() error {
            return pool.Add(org.ID, generateNKeyForOrg(org))
        })
    }
    if err := g.Wait(); err != nil {
        return fmt.Errorf("org connection failed: %w", err)
    }

    // 4. HTTP server starts ONLY after all orgs connected
    slog.Info("boot complete", "orgs", len(orgs))
    return nil
}
```

If NATS is unavailable at boot, notifd exits immediately (fail fast, let systemd/k8s restart).
Individual org connection failures are retried with backoff, but boot blocks until all succeed.

### Monitoring

System connection provides cluster-wide observability:

```go
// $SYS.REQ.ACCOUNT.{pubkey}.INFO — per-account metrics
// $SYS.SERVER.ACCOUNT.{pubkey}.CONNS — connection count
// $SYS.REQ.SERVER.PING — cluster health
```

Key metrics to export (Prometheus/OTEL):
- `notifd_accounts_total` — number of active accounts
- `notifd_connections_per_account` — connections per org
- `notifd_jwt_push_duration_seconds` — JWT rebuild+push latency
- `notifd_jwt_push_errors_total` — failed pushes
- `notifd_stream_messages_total{org,stream}` — per-org stream throughput
- `notifd_boot_duration_seconds` — time from start to HTTP ready

Alert rules:
- JWT push latency > 5s → warning
- JWT push error → critical (investigate NATS connectivity)
- Account connection count = 0 for > 60s → warning (org may be orphaned)

## Implementation Plan

### Phase 0: Audit Log Infrastructure

1. Create `audit_log` table migration
2. Implement `internal/audit/audit.go` — structured audit logging to DB + slog
3. Add `notif audit` CLI command for querying audit log
4. Document OPERATOR_SEED rotation runbook
5. Wire audit logging into existing API handlers as baseline

### Phase 1: Orgs Table + Foundation

1. Create `orgs` table migration
2. Backfill existing org_id strings into orgs table
3. Add FKs from projects, api_keys to orgs
4. Add `nats_public_key` and `billing_tier` columns

### Phase 2: Operator + Resolver Setup

1. Generate operator NKey pair (one-time, stored as secret)
2. Configure NATS server with `nats-resolver` full type
3. Create system account programmatically (`nats-io/jwt/v2`)
4. notifd boots with system connection for `$SYS.>` only
5. Push bootstrap JWTs via `$SYS.REQ.CLAIMS.UPDATE`

### Phase 3: ClientPool + Per-Account Streams

1. Implement `internal/nats/pool.go` — ClientPool with system + per-account connections
2. Refactor `EnsureStreams` → `EnsureStreamsForOrg(orgID)`
3. On org create: generate account JWT → push → create OrgClient → ensure streams
4. On org delete: revoke JWT → push → remove OrgClient → streams GC'd by NATS
5. HTTP server resolves orgID → `pool.Get(orgID)` instead of single `nc`

### Phase 4: Account Lifecycle API

1. `notif org create <name>` → insert into orgs table → generate NATS account JWT → push
2. `notif org delete <name>` → revoke JWT → cascade (projects, api_keys, federation)
3. `notif org list` → list from orgs table
4. `notif org limits <name>` → show/set account resource limits

### Phase 5: User Provisioning

1. On org create, auto-create `notifd-{orgID}` user (for ClientPool connection)
2. On org create, auto-create `api-user` (for CLI/SDK connections)
3. On `notif connect`, create `leafnode-user` with NKey credentials
4. Key rotation: regenerate user NKey pair, sign new JWT, push

### Phase 6: Billing Integration

1. NATS account limits map to `orgs.billing_tier`
2. Usage metrics from system connection ($SYS) → billing system
3. Tier upgrade/downgrade: regenerate account JWT with new limits → push

## File Structure

```
internal/audit/
  audit.go                # Structured audit logging (DB + slog)

internal/nats/
  pool.go                 # ClientPool (system + per-account connections)
  client.go               # OrgClient (refactored from current Client)
  publisher.go            # unchanged (uses OrgClient)
  consumer.go             # unchanged (uses OrgClient)
  reader.go               # unchanged (uses OrgClient)
  dlq.go                  # unchanged (uses OrgClient)

internal/accounts/
  accounts.go             # Org/Account lifecycle (create/delete/list)
  jwt.go                  # RebuildAndPushAccountJWT + transactional multi-push
  keys.go                 # NKey generation via nats-io/nkeys (ephemeral, in-memory)
  limits.go               # Billing tier → NATS limits mapping

# NATS server config
nats-server.conf
  resolver: nats                 # nats-resolver full
  system_account: <SYS_PUBKEY>
  resolver_preload: { ... }      # bootstrap system account JWT only
```

## Dependencies

- `github.com/nats-io/nkeys` — NKey pair generation
- `github.com/nats-io/jwt/v2` — JWT creation and signing
- **Blocks**: `federation-cli` (can't federate without accounts)
- **Blocks**: billing-by-tenant (can't meter without account boundaries)
- **Blocked by**: nothing (greenfield, zero existing tenants)

## Execution Order

```
1. Audit log infra + operator seed rotation runbook    ← Phase 0
2. nats-accounts (Phases 1-6)                          ← this design
3. notif connect (leaf-node only)                      ← notifd-cloud-bridge
4. Validate 1-2-3 end-to-end                           ← GATE: no federation until validated
5. Federation CLI (when demand exists)                  ← federation-cli
```

## Acceptance Criteria

### Phase 0 (Audit + Operations)
- [ ] `audit_log` table exists with structured logging for all security operations
- [ ] `notif audit` CLI queries audit log (by org, action, time range)
- [ ] OPERATOR_SEED rotation runbook documented and tested
- [ ] Every JWT push, credential provision, and tier change is audit logged

### Core (Phases 1-6)
- [ ] notifd boots with operator + system account via nats-resolver full
- [ ] Boot sequence: HTTP blocked until all org accounts connected (fail fast if NATS down)
- [ ] System connection used ONLY for $SYS monitoring and claims management
- [ ] `notif org create <name>` provisions NATS account via Go JWT generation (no nsc)
- [ ] `notif org delete <name>` revokes account JWT and cascades (projects, keys, federation)
- [ ] Each org gets its own streams: NOTIF_EVENTS_{orgID}, NOTIF_DLQ_{orgID}, NOTIF_WEBHOOK_RETRY_{orgID}
- [ ] ClientPool: HTTP request → resolve orgID → per-account connection → publish
- [ ] Tenant A cannot subscribe to Tenant B's subjects (protocol-level isolation)
- [ ] Account-level resource limits configurable per billing tier
- [ ] Self-hosted leaf node connects with account-scoped NKey credentials
- [ ] Zero-downtime: adding/removing accounts does not restart NATS server
- [ ] JWT push via `$SYS.REQ.CLAIMS.UPDATE` (no nsc subprocess)
- [ ] JWT-as-derived-view: always rebuilt from DB state, never stored or loaded
- [ ] Transactional multi-account JWT push (both succeed or rollback)
- [ ] NKey credentials regenerated on restart (in-memory only, no persistent secrets)
- [ ] Only persistent secret is OPERATOR_SEED (env var or file mount)
- [ ] `orgs` table is source of truth with nats_public_key and billing_tier
- [ ] Monitoring: per-account metrics exported (connections, throughput, JWT latency)
