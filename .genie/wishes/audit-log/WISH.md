# Wish: Audit Log Infrastructure

**Status:** DRAFT
**Slug:** `audit-log`
**Created:** 2026-02-18

---

## Summary

Add structured audit logging to notifd as a prerequisite for all security-sensitive features (NATS accounts, credential provisioning, federation). Every JWT push, credential issue, and account lifecycle event must be recorded with actor, action, target, and metadata.

---

## Dependencies

- **depends-on:** nothing (greenfield)
- **blocks:** `nats-accounts`, `notif-connect-phase1` (Phase 2 credential provisioning)

---

## Scope

### IN
- `audit_log` Postgres table with indexes
- `internal/audit/audit.go` — Go package for structured audit logging (DB + slog dual-write)
- `notif audit` CLI command (query by org, action, time range)
- OPERATOR_SEED rotation runbook (documented, not automated)
- Wire audit logging into existing API handlers as baseline proof

### OUT
- Automated OPERATOR_SEED rotation (manual runbook only)
- External log aggregation integration (slog output is sufficient)
- Audit log UI/dashboard (CLI only for now)
- Retention policies or audit log pruning (manual for now)
- Alerting on audit events (deferred to monitoring layer)

---

## Decisions

- **DEC-1:** Dual-write to Postgres + slog. DB for queryability, slog for external aggregation. If DB write fails, slog still captures the event (never lose audit data).
- **DEC-2:** Audit log table uses BIGSERIAL (not UUID). Audit events are append-only, monotonically ordered. Sequential ID is simpler and faster.
- **DEC-3:** OPERATOR_SEED rotation is a documented manual runbook, not an automated CLI. Rotation frequency is annual or on compromise. Automation adds complexity for a rare operation.
- **DEC-4:** `ip_address` field is INET type (Postgres native). Captures client IP for API/CLI actions. "notifd" internal actions have NULL IP.

---

## Success Criteria

- [ ] `audit_log` table exists with `org_id`, `action`, `timestamp` indexes
- [ ] `internal/audit` package provides `Log(ctx, action, orgID, target, detail)` function
- [ ] Every existing API handler (emit, subscribe, webhook) calls audit.Log
- [ ] `notif audit` CLI lists events, filterable by `--org`, `--action`, `--since`
- [ ] OPERATOR_SEED rotation runbook is documented in `docs/operator-seed-rotation.md`
- [ ] Audit events also appear in slog output (structured JSON)

---

## Assumptions

- **ASM-1:** Postgres is already available (notifd uses it for webhooks, events, etc.)
- **ASM-2:** slog is already configured in notifd (JSON format)
- **ASM-3:** Audit volume is low enough that Postgres handles it without partitioning (< 1M rows/month initially)

## Risks

- **RISK-1:** Audit log DB writes add latency to hot paths (emit, webhook). — Mitigation: async insert via channel+goroutine. Log to slog synchronously, DB insert is best-effort async.
- **RISK-2:** Audit log grows unbounded. — Mitigation: add `created_at` index for future retention policy. Not urgent at current scale.

---

## Execution Groups

### Group A: Audit Log Table + Package

**Goal:** Create the audit_log table and Go package for recording events.

**Deliverables:**
- Postgres migration: `CREATE TABLE audit_log` with indexes
- `internal/audit/audit.go` — `Log()` function with async DB insert + sync slog
- `internal/audit/audit_test.go` — unit tests for Log function

**Acceptance Criteria:**
- [ ] Migration runs cleanly on existing notifd database
- [ ] `audit.Log(ctx, "test.action", "org_1", "target_1", map[string]any{"key": "val"})` inserts row
- [ ] Same call also emits slog event with matching fields
- [ ] DB insert failure does not block the caller (async)

**Validation:** `go test ./internal/audit/... -v`

---

### Group B: Wire Audit into Existing Handlers

**Goal:** Add audit logging to all existing API handlers as baseline.

**Deliverables:**
- Audit calls in emit handler (`event.emit`)
- Audit calls in webhook CRUD handlers (`webhook.create`, `webhook.delete`)
- Audit calls in subscribe handler (`subscription.create`)
- Audit calls in API key handlers if they exist

**Acceptance Criteria:**
- [ ] `POST /api/v1/events` logs `event.emit` with org_id and topic
- [ ] Webhook create/delete logs `webhook.create` / `webhook.delete`
- [ ] All audit entries include IP address from request context

**Validation:** `curl -X POST .../api/v1/events ... && notif audit --since 1m`

---

### Group C: Audit CLI

**Goal:** `notif audit` command for querying the audit log.

**Deliverables:**
- `cmd/notif/audit.go` or equivalent CLI subcommand
- Flags: `--org <id>`, `--action <action>`, `--since <duration>`, `--limit <n>`
- Human-readable table output (default), `--json` for machine-readable

**Acceptance Criteria:**
- [ ] `notif audit` shows recent events in table format
- [ ] `notif audit --org org_1 --since 1h` filters correctly
- [ ] `notif audit --action event.emit --limit 10` works
- [ ] `notif audit --json` outputs JSON array

**Validation:** `notif audit --since 5m --json | jq length`

---

### Group D: OPERATOR_SEED Rotation Runbook

**Goal:** Document the procedure for rotating the operator signing key.

**Deliverables:**
- `docs/operator-seed-rotation.md` with step-by-step procedure
- `notif accounts rebuild-all` CLI command (used during rotation)
- `notif accounts verify-all` CLI command (post-rotation verification)

**Acceptance Criteria:**
- [ ] Runbook covers: generate new key, update NATS config, rebuild JWTs, rotate env, restart, verify
- [ ] `rebuild-all` iterates orgs table and calls RebuildAndPush for each
- [ ] `verify-all` checks every account JWT signature against current operator

**Validation:** `cat docs/operator-seed-rotation.md | wc -l` (non-empty) + `go build ./cmd/notif/...`

---

## Review Results

_Populated by `/review` after execution completes._

---

## Files to Create/Modify

```
# New
internal/audit/audit.go              # Audit logging package
internal/audit/audit_test.go         # Tests
docs/operator-seed-rotation.md       # Rotation runbook

# New (migrations)
migrations/NNNN_create_audit_log.sql # audit_log table

# Modified
internal/server/handlers.go          # Wire audit.Log into existing handlers
cmd/notif/audit.go                   # CLI subcommand (or equivalent)
cmd/notif/accounts.go                # rebuild-all, verify-all commands
```
