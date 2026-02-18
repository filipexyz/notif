# DREAM: notif.sh Cloud Platform Foundation

**Date:** 2026-02-18
**Base branch:** `main`
**Team:** `dream-2026-02-18`

---

## Wishes (topological order)

### merge_order: 1 (parallel — no dependencies)

#### audit-log

- **slug:** `audit-log`
- **branch:** `feat/audit-log`
- **worktree_path:** `/tmp/dream-worktrees/audit-log`
- **wish_path:** `.genie/wishes/audit-log/WISH.md`
- **depends_on:** []
- **merge_order:** 1
- **worker_prompt:** |
    You are implementing the `audit-log` wish for the notif.sh project.

    **Context:**
    - Repository: notif.sh (Go + Postgres + NATS event notification platform)
    - Wish file: `.genie/wishes/audit-log/WISH.md`
    - Branch: `feat/audit-log`
    - Worktree: `/tmp/dream-worktrees/audit-log`
    - Base: `main`

    **Instructions:**
    1. Read the WISH.md at the wish_path above for full requirements.
    2. Implement ALL execution groups (A through D) as specified.
    3. Key deliverables:
       - Postgres migration for `audit_log` table with indexes
       - `internal/audit/audit.go` — Log() function with async DB insert + sync slog
       - Wire audit.Log into existing HTTP handlers (emit, webhook, subscribe)
       - `notif audit` CLI command with filtering (--org, --action, --since, --json)
       - OPERATOR_SEED rotation runbook at `docs/operator-seed-rotation.md`
       - `notif accounts rebuild-all` and `verify-all` CLI commands
    4. Run tests: `go test ./... -v`
    5. If tests fail, fix and retry (max 3 attempts).
    6. When CI green, create PR: `gh pr create --base main`

    **Report format:**
    - Success: `DONE: PR at <url>. CI: green. Groups: N/N.`
    - Failure: `BLOCKED: <reason>. Groups: N/N.`

---

#### notif-connect-phase1

- **slug:** `notif-connect-phase1`
- **branch:** `feat/notif-connect-phase1`
- **worktree_path:** `/tmp/dream-worktrees/notif-connect-phase1`
- **wish_path:** `.genie/wishes/notif-connect-phase1/WISH.md`
- **depends_on:** []
- **merge_order:** 1
- **worker_prompt:** |
    You are implementing the `notif-connect-phase1` wish for the notif.sh project.

    **Context:**
    - Repository: notif.sh (Go + Postgres + NATS event notification platform)
    - Wish file: `.genie/wishes/notif-connect-phase1/WISH.md`
    - Branch: `feat/notif-connect-phase1`
    - Worktree: `/tmp/dream-worktrees/notif-connect-phase1`
    - Base: `main`

    **Instructions:**
    1. Read the WISH.md at the wish_path above for full requirements.
    2. Implement ALL execution groups (A through D) as specified.
    3. Key deliverables:
       - `cmd/notif/connect.go` — cobra subcommand tree (install/start/stop/status/logs/uninstall/run)
       - `internal/bridge/bridge.go` — Bridge runtime (NATS connection, stream, interceptors)
       - `internal/bridge/service.go` — kardianos/service integration
       - `internal/bridge/config.go` — Config resolution (flags → env → file) + drift detection
       - `internal/bridge/status.go` — Status reporting (heartbeat 10s, stale 30s, dead 60s)
       - Log rotation via lumberjack
       - Stream conflict detection
    4. Add dependencies: `go get github.com/kardianos/service gopkg.in/natefinch/lumberjack.v2`
    5. Run tests: `go test ./... -v`
    6. If tests fail, fix and retry (max 3 attempts).
    7. When CI green, create PR: `gh pr create --base main`

    **Report format:**
    - Success: `DONE: PR at <url>. CI: green. Groups: N/N.`
    - Failure: `BLOCKED: <reason>. Groups: N/N.`

---

### merge_order: 2 (depends on audit-log)

#### nats-accounts

- **slug:** `nats-accounts`
- **branch:** `feat/nats-accounts`
- **worktree_path:** `/tmp/dream-worktrees/nats-accounts`
- **wish_path:** `.genie/wishes/nats-accounts/WISH.md`
- **depends_on:** [audit-log]
- **merge_order:** 2
- **worker_prompt:** |
    You are implementing the `nats-accounts` wish for the notif.sh project.

    **Context:**
    - Repository: notif.sh (Go + Postgres + NATS event notification platform)
    - Wish file: `.genie/wishes/nats-accounts/WISH.md`
    - Branch: `feat/nats-accounts`
    - Worktree: `/tmp/dream-worktrees/nats-accounts`
    - Base: `feat/audit-log` (must include audit-log changes)
    - IMPORTANT: This wish depends on `audit-log` being merged first. Base off that branch.

    **Instructions:**
    1. Read the WISH.md at the wish_path above for full requirements.
    2. Implement ALL execution groups (A through F) as specified.
    3. Key deliverables:
       - Migration: `orgs` table with nats_public_key, billing_tier
       - Migration: backfill existing org_ids, add FKs
       - `internal/accounts/jwt.go` — RebuildAndPushAccountJWT + transactional multi-push
       - `internal/accounts/keys.go` — NKey generation (ephemeral, in-memory)
       - `internal/accounts/accounts.go` — Org/Account lifecycle
       - `internal/accounts/limits.go` — Billing tier → NATS limits
       - `internal/nats/pool.go` — ClientPool (system + per-account connections)
       - Refactor `internal/nats/client.go` → OrgClient
       - Updated `nats-server.conf` with resolver: nats, system_account
       - `notif org create/delete/list/limits` CLI
       - Refactor all HTTP handlers to use pool.Get(orgID)
       - Monitoring metrics (Prometheus/OTEL)
       - Boot sequence: block HTTP until all orgs connected
    4. Add dependencies: `go get github.com/nats-io/nkeys github.com/nats-io/jwt/v2`
    5. Run tests: `go test ./... -v`
    6. If tests fail, fix and retry (max 3 attempts).
    7. When CI green, create PR: `gh pr create --base feat/audit-log`

    **Report format:**
    - Success: `DONE: PR at <url>. CI: green. Groups: N/N.`
    - Failure: `BLOCKED: <reason>. Groups: N/N.`

---

## Execution Plan

```
Layer 1 (parallel):
  ├── audit-log            → feat/audit-log        (base: main)
  └── notif-connect-phase1 → feat/notif-connect     (base: main)

Layer 2 (after audit-log merges):
  └── nats-accounts        → feat/nats-accounts    (base: feat/audit-log)
```

## Notes

- `federation-cli` is DEFERRED — design complete, implementation waits for demand validation after E2E testing of layers 1-3.
- `notif-connect-phase1` is Phase 1 only (local-only). Phase 2 (cloud/leaf node) depends on `nats-accounts`.
- Workers should use `/refine` on their prompt before executing.
