# Wish: NATS Accounts (Account-per-Tenant)

**Status:** DRAFT
**Slug:** `nats-accounts`
**Created:** 2026-02-18

---

## Summary

Migrate notifd from single-account NATS to multi-account isolation via NATS Accounts. Each org gets its own NATS account with protocol-level isolation, per-account streams, and billing-granular resource limits. Uses `nats-resolver full` with programmatic JWT generation (no nsc), ClientPool for per-account connections, and JWT-as-derived-view pattern.

---

## Dependencies

- **depends-on:** `audit-log` (audit infrastructure must exist before JWT operations)
- **blocks:** `federation-cli` (can't federate without accounts)

---

## Scope

### IN
- `orgs` table (central entity with nats_public_key, billing_tier)
- Operator + system account bootstrap via `nats-resolver full`
- Programmatic JWT generation (`nats-io/nkeys` + `nats-io/jwt/v2`)
- `RebuildAndPushAccountJWT` (JWT-as-derived-view, always from DB)
- `RebuildAndPushMultipleAccounts` (transactional multi-account push with rollback)
- `ClientPool` with system + per-account connections
- Per-account streams: `NOTIF_EVENTS_{orgID}`, `NOTIF_DLQ_{orgID}`, `NOTIF_WEBHOOK_RETRY_{orgID}`
- Boot sequence: block HTTP until all org accounts connected
- `notif org create/delete/list/limits` CLI
- NKey credentials regenerated on restart (in-memory only)
- Monitoring: Prometheus/OTEL metrics for accounts, connections, JWT operations
- Account lifecycle API (provision/delete/limits)

### OUT
- Federation (separate wish: `federation-cli`, deferred)
- Leaf node provisioning for self-hosted (separate wish: `notif-connect-phase1`)
- Billing system integration beyond tier→limits mapping
- NATS clustering (single server for now)
- Automated OPERATOR_SEED rotation (manual runbook in `audit-log` wish)
- User-facing org management UI (CLI only)

---

## Decisions

- **DEC-1:** `nats-resolver full` — dynamic, self-contained in JetStream, programmatic push. Not memory (static), url (legacy), or nsc (fragile subprocess).
- **DEC-2:** Programmatic JWT via `nats-io/nkeys` + `jwt/v2` — single binary, no persistent volume, concurrency safe, testable.
- **DEC-3:** JWT-as-derived-view — never stored, always rebuilt from DB state. Eliminates JWT↔DB desync risk. Idempotent rebuild.
- **DEC-4:** ClientPool with per-account connections — no god-connection. System connection for `$SYS` only. Each org's `notifd-{orgID}` user connects independently.
- **DEC-5:** NKey credentials regenerated on restart — in-memory only, no persistent credential storage. Only OPERATOR_SEED is persistent.
- **DEC-6:** Per-account streams — 3 streams per org (events, dlq, webhook-retry). Stream limits tied to billing tier.
- **DEC-7:** Transactional multi-account JWT push — for operations affecting multiple accounts (federation), both JWTs succeed or rollback. Single-account ops use simple push.
- **DEC-8:** Boot blocks HTTP — notifd doesn't accept traffic until all org accounts connected. Fail fast if NATS unavailable.

---

## Success Criteria

- [ ] notifd boots with operator + system account via nats-resolver full
- [ ] Boot sequence blocks HTTP until all org accounts connected
- [ ] `notif org create <name>` provisions NATS account via Go JWT generation
- [ ] `notif org delete <name>` revokes account JWT and cascades
- [ ] Each org gets 3 per-account streams
- [ ] ClientPool: HTTP request → resolve orgID → per-account connection → publish
- [ ] Tenant A cannot subscribe to Tenant B's subjects (protocol isolation)
- [ ] Account limits configurable per billing tier
- [ ] Zero-downtime: adding/removing accounts does not restart NATS
- [ ] JWT-as-derived-view: always rebuilt from DB, never stored
- [ ] Transactional multi-account JWT push works
- [ ] NKey credentials regenerated on restart
- [ ] All JWT operations audit logged
- [ ] Monitoring metrics exported

---

## Assumptions

- **ASM-1:** Zero existing tenants in NATS (greenfield migration — no live traffic to migrate)
- **ASM-2:** Single NATS server (no clustering needed initially)
- **ASM-3:** Postgres already available for orgs table
- **ASM-4:** `audit-log` wish completed (audit infrastructure available)

## Risks

- **RISK-1:** Boot time increases with org count (parallel connections per org). — Mitigation: parallel connect with errgroup. Expected ~100ms per org, 100 orgs ≈ 1-2s.
- **RISK-2:** OPERATOR_SEED compromise = total system compromise. — Mitigation: env var/secret mount, rotation runbook, audit log every use.
- **RISK-3:** Per-account streams multiply storage. — Mitigation: tier-based limits (free: 256MB). At 100 orgs × 256MB = 25GB total.
- **RISK-4:** JWT rebuild latency at scale (10k+ orgs). — Mitigation: ~1-2ms per JWT build. RebuildAll for rotation: 10k × 2ms = 20s. Acceptable for rare operation.

---

## Execution Groups

### Group A: Orgs Table + Foundation

**Goal:** Create the central orgs entity and migrate existing org_id references.

**Deliverables:**
- Migration: `CREATE TABLE orgs` with id, name, external_id, nats_public_key, billing_tier
- Migration: backfill existing org_id strings into orgs table
- Migration: add FKs from projects, api_keys to orgs
- `internal/db/orgs.go` — CRUD for orgs table

**Acceptance Criteria:**
- [ ] `orgs` table exists with all columns
- [ ] Existing org_id strings backfilled
- [ ] FK constraints from projects and api_keys to orgs work
- [ ] `db.CreateOrg()`, `db.GetOrg()`, `db.ListOrgs()`, `db.DeleteOrg()` work

**Validation:** `go test ./internal/db/... -run TestOrgs -v`

---

### Group B: Operator + Resolver Setup

**Goal:** Configure NATS with nats-resolver full and system account.

**Deliverables:**
- One-time operator NKey pair generation (stored as OPERATOR_SEED secret)
- Updated `nats-server.conf` with resolver: nats, system_account, resolver_preload
- `internal/accounts/jwt.go` — `RebuildAndPushAccountJWT` function
- `internal/accounts/keys.go` — NKey generation utilities
- Bootstrap system account JWT on first boot

**Acceptance Criteria:**
- [ ] NATS starts with resolver type `full`
- [ ] System account JWT pushed via `$SYS.REQ.CLAIMS.UPDATE`
- [ ] notifd connects as system user to `$SYS.>`
- [ ] `RebuildAndPushAccountJWT` builds JWT from DB state and pushes
- [ ] JWT push audit logged

**Validation:** `go test ./internal/accounts/... -v` + `nats account info` against running NATS

---

### Group C: ClientPool + Per-Account Streams

**Goal:** Replace single `*nats.Conn` with per-account ClientPool.

**Deliverables:**
- `internal/nats/pool.go` — ClientPool struct with system + per-account connections
- `internal/nats/client.go` — refactored OrgClient (from current Client)
- `EnsureStreamsForOrg(orgID)` — creates 3 streams per org
- Boot sequence: connect system → load orgs → parallel connect all → start HTTP

**Acceptance Criteria:**
- [ ] `pool.Get(orgID)` returns per-account connection
- [ ] `pool.Add(orgID)` generates NKey, creates user JWT, connects
- [ ] `pool.Remove(orgID)` disconnects and cleans up
- [ ] 3 streams created per org on Add
- [ ] HTTP server blocks until boot completes
- [ ] Fail fast if NATS unavailable at boot

**Validation:** `go test ./internal/nats/... -v` + boot notifd against test NATS with 2+ orgs

---

### Group D: Account Lifecycle API + CLI

**Goal:** CRUD for org accounts via API and CLI.

**Deliverables:**
- `POST /api/v1/orgs` — create org (generates NATS account)
- `DELETE /api/v1/orgs/:id` — delete org (revokes account, cascades)
- `GET /api/v1/orgs` — list orgs
- `PUT /api/v1/orgs/:id/limits` — set account limits
- `notif org create/delete/list/limits` CLI commands
- Transactional JWT push (`RebuildAndPushMultipleAccounts`)
- `internal/accounts/limits.go` — billing tier → NATS limits mapping

**Acceptance Criteria:**
- [ ] `notif org create test-org` inserts into DB, generates account JWT, pushes, creates pool connection
- [ ] `notif org delete test-org` revokes JWT, removes pool connection, cascades FK
- [ ] `notif org list` shows all orgs with status
- [ ] `notif org limits test-org` shows/sets account limits from billing tier
- [ ] All operations audit logged
- [ ] Transactional push: multi-account operations roll back on failure

**Validation:** `notif org create test && notif org list && notif org delete test`

---

### Group E: HTTP Refactor (orgID → ClientPool)

**Goal:** Replace all HTTP handlers using single `nc` with ClientPool.

**Deliverables:**
- Middleware: resolve orgID from API key → `pool.Get(orgID)`
- Refactor emit handler to use OrgClient
- Refactor subscribe handler to use OrgClient
- Refactor webhook handler to use OrgClient
- Refactor DLQ handler to use OrgClient

**Acceptance Criteria:**
- [ ] `POST /api/v1/events` publishes to org-specific NATS account
- [ ] Org A's API key cannot publish to Org B's streams (protocol enforcement)
- [ ] All existing API tests pass with ClientPool
- [ ] No single `*nats.Conn` remaining in HTTP handlers

**Validation:** `go test ./internal/server/... -v` + manual test: emit with org_a key, verify isolation

---

### Group F: Monitoring + Metrics

**Goal:** Export per-account metrics for observability.

**Deliverables:**
- Prometheus/OTEL metrics: accounts_total, connections_per_account, jwt_push_duration, jwt_push_errors, stream_messages, boot_duration
- Health check endpoint: `/healthz` reports pool status
- `$SYS` monitoring queries via system connection

**Acceptance Criteria:**
- [ ] `/metrics` endpoint exports all defined metrics
- [ ] `notifd_jwt_push_duration_seconds` histogram populated on JWT push
- [ ] `notifd_boot_duration_seconds` recorded
- [ ] `/healthz` returns unhealthy if any org disconnected

**Validation:** `curl localhost:8080/metrics | grep notifd_accounts_total`

---

## Review Results

_Populated by `/review` after execution completes._

---

## Files to Create/Modify

```
# New
internal/accounts/jwt.go             # RebuildAndPush + transactional multi-push
internal/accounts/keys.go            # NKey generation (ephemeral, in-memory)
internal/accounts/accounts.go        # Org/Account lifecycle
internal/accounts/limits.go          # Billing tier → NATS limits
internal/nats/pool.go                # ClientPool (system + per-account)
internal/db/orgs.go                  # Orgs table CRUD

# New (migrations)
migrations/NNNN_create_orgs.sql      # orgs table
migrations/NNNN_orgs_backfill.sql    # backfill existing org_ids
migrations/NNNN_orgs_fks.sql         # FK constraints

# Modified
internal/nats/client.go              # Refactor to OrgClient
nats-server.conf                     # resolver: nats, system_account
internal/server/handlers.go          # Use pool.Get(orgID) instead of single nc
internal/server/middleware.go         # Resolve orgID → OrgClient
cmd/notif/org.go                     # CLI subcommands
```
