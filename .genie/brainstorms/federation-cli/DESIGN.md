# Design: Federation CLI (Account↔Account)

> NATS native exports/imports between accounts in the same cloud cluster.
>
> **Implementation status: DEFERRED.** Design is complete. Implementation waits
> until nats-accounts + notif connect are validated end-to-end and demand exists.

## Problem

With NATS Accounts in cloud, tenants need a way to share subjects between each
other (e.g., Org A exports `orders.>` to Org B). The federation CLI provides
NATS-native export/import between accounts with a simple CLI/API interface.

## User Journey

```bash
# Org A: export orders to Org B
notif federation export orders.> --to org-b --name "order-feed"
# → Export created (pending import from org-b)
# → Notification sent to org-b admins

# Org B: import orders from Org A (sees pending request in dashboard)
notif federation import orders.> --from org-a --as vendor-orders.>
# → Import active! Events flowing via NATS export/import.

# List active federation
notif federation ls
# NAME         TYPE    SUBJECT    PARTNER  STATUS  MAPPED AS
# order-feed   export  orders.>   org-b    active  -
# vendor-feed  import  orders.>   org-a    active  vendor-orders.>

# Revoke
notif federation rm order-feed
# → Export revoked. Flow stopped immediately.
```

## Architecture

```
┌─────────────────────────────────────────────────┐
│  NATS Cluster (multi-account)                   │
│                                                 │
│  ┌─ Account: org_a ─────────────────────┐       │
│  │  events.org_a.*.orders.>             │       │
│  │  EXPORT: orders.> → org_b            │───┐   │
│  └──────────────────────────────────────┘   │   │
│                                             │   │
│  ┌─ Account: org_b ─────────────────────┐   │   │
│  │  IMPORT: orders.> from org_a         │◄──┘   │
│  │  → remapped as vendor-orders.>       │       │
│  └──────────────────────────────────────┘       │
│                                                 │
│  ┌─ System Account (notifd) ────────────┐       │
│  │  Manages exports/imports via jwt/v2   │       │
│  │  Stores metadata in Postgres          │       │
│  └──────────────────────────────────────┘       │
└─────────────────────────────────────────────────┘
```

NATS-native exports/imports only. No HTTP/WS bridge layer.
The `internal/federation/client.go` (WS subscribe + HTTP emit) is a separate
concern for cross-cluster federation (if ever needed). This design is
intra-cluster, account↔account.

## Key Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Mechanism | NATS native exports/imports | Zero-copy within cluster. Protocol-level. NATS handles delivery, backpressure, ordering. |
| Export type | Stream (pub/sub) | Events are pub/sub. Service exports (req/reply) is stretch goal. |
| Authorization | Bilateral consent | Org A creates export, Org B creates import. Both must exist for flow to activate. Prevents unilateral data leak. |
| Subject mapping | Optional, on import side | Importer decides remapping. Default: same subject name. Keeps exporter simple. |
| Managed by | notifd (system account) | System account manages account JWTs. notifd regenerates account JWT with export/import claims, pushes via `$SYS.REQ.CLAIMS.UPDATE`. |
| State storage | Postgres (orgs table + federation tables) | Federation metadata (who, what, when, status) lives in DB. NATS JWTs are enforcement, DB is source of truth for UI/API. |
| Revocation | Immediate | Remove export/import → regenerate JWT without claim → push → NATS cuts flow immediately. |
| JWT management | Programmatic via `nats-io/jwt/v2` | Same as nats-accounts. No nsc subprocess. Account JWT regenerated with export/import claims added/removed. |
| Export validation | Reject naked `>` only | Only naked `>` (export everything) is rejected. Single-level wildcards like `orders.>` are valid use cases. |
| Discovery | Query-based + notif events | No dedicated notifications table. Pending exports queried directly from `federation_exports`. Async notification via notif event (`federation.export.created`). |
| Pending export TTL | 7 days auto-revoke | Pending exports (no matching import) auto-revoke after 7 days. Prevents stale pending requests. Cron job or DB check on access. |

## Activation Flow

```
1. Org A creates export                   → status: pending
   └─ Notif event emitted: federation.export.created (target org can subscribe)
   └─ Visible in Org B via `notif federation ls --pending`
   └─ Auto-revokes after 7 days if no matching import

2. Org B creates matching import          → status: pending

3. notifd detects match → TRANSACTIONAL activation:
   a. Build org_a JWT with export claim (in memory, not pushed yet)
   b. Build org_b JWT with import claim (in memory, not pushed yet)
   c. Push org_a JWT via $SYS.REQ.CLAIMS.UPDATE
   d. Push org_b JWT via $SYS.REQ.CLAIMS.UPDATE
   e. If BOTH succeed → both statuses → active
   f. If EITHER fails → rollback pushed JWTs, both statuses stay pending
      (Rollback: rebuild and push the accounts without the new claims)

4. Events flow via NATS native export/import (zero-copy)

Revocation:
1. Either party calls `federation rm`
2. notifd: transactional push (both accounts rebuilt without claims)
3. NATS cuts flow immediately (JWT update is atomic)
4. Both statuses → revoked

Org Deletion Cascade:
1. Org A deleted → `ON DELETE CASCADE` revokes all exports from org_a
2. For each active import referencing org_a exports:
   a. Import status → `terminated` (distinct from `revoked`)
   b. Importing org's JWT rebuilt without the dead import
   c. Notification: `federation.source.deleted` event emitted to importing org
3. Zombie imports (source org gone) are visible via `notif federation ls --terminated`
```

## Implementation

### DB Schema

```sql
-- Depends on: orgs table from nats-accounts design

CREATE TABLE federation_exports (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      VARCHAR(32) NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    subject     TEXT NOT NULL,
    target_org  VARCHAR(32) NOT NULL REFERENCES orgs(id),
    status      TEXT DEFAULT 'pending',  -- pending | active | revoked | expired
    created_at  TIMESTAMPTZ DEFAULT now(),
    expires_at  TIMESTAMPTZ DEFAULT now() + INTERVAL '7 days',  -- auto-revoke if no import
    revoked_at  TIMESTAMPTZ,
    UNIQUE(org_id, name),
    CHECK (subject != '>')            -- only reject naked > (export everything)
);

CREATE TABLE federation_imports (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        VARCHAR(32) NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    export_id     UUID NOT NULL REFERENCES federation_exports(id) ON DELETE CASCADE,
    local_subject TEXT,                  -- remapped subject (null = same as export)
    status        TEXT DEFAULT 'pending', -- pending | active | revoked | terminated
    created_at    TIMESTAMPTZ DEFAULT now(),
    revoked_at    TIMESTAMPTZ
);

-- No federation_notifications table. Pending exports are queried directly:
--   SELECT * FROM federation_exports WHERE target_org = $1 AND status = 'pending'
-- Async notification via notif event: federation.export.created
```

### API

```
POST   /api/v1/federation/exports            # Create export
GET    /api/v1/federation/exports            # List exports (own org)
DELETE /api/v1/federation/exports/:id        # Revoke export

POST   /api/v1/federation/imports            # Create import
GET    /api/v1/federation/imports            # List imports (own org)
DELETE /api/v1/federation/imports/:id        # Revoke import

GET    /api/v1/federation/pending            # Pending requests targeting this org
POST   /api/v1/federation/pending/:id/accept # Accept = create import
```

### CLI

```bash
notif federation export <subject> --to <org> --name <name>
notif federation import <subject> --from <org> [--as <local-subject>]
notif federation ls [--exports | --imports | --pending | --terminated]
notif federation rm <name-or-id>
notif federation accept <export-id> [--as <local-subject>]
notif federation status <name-or-id>    # consumer lag, flow health, throughput
```

### JWT Operations (JWT-as-Derived-View)

Federation operations use the same `RebuildAndPushAccountJWT` from nats-accounts.
JWTs are never loaded or modified — always rebuilt from DB state.

Export creation flow:
```go
// 1. Insert export row in DB
db.CreateFederationExport(orgA, "order-feed", "orders.>", orgB)

// 2. Rebuild org_a's JWT from complete DB state (includes new export)
accounts.RebuildAndPushAccountJWT(db, orgA, operatorKP, sysConn)

// 3. If matching import exists, also rebuild org_b's JWT
if match := db.FindMatchingImport(exportID); match != nil {
    accounts.RebuildAndPushAccountJWT(db, orgB, operatorKP, sysConn)
    db.ActivateFederation(exportID, match.ID)
}
```

Import creation flow (transactional):
```go
// 1. Insert import row in DB
db.CreateFederationImport(orgB, exportID, "vendor-orders.>")

// 2. Transactional push: both accounts or neither
err := accounts.RebuildAndPushMultipleAccounts(db, []string{orgA, orgB}, operatorKP, sysConn)
if err != nil {
    // Rollback: delete import row, both accounts stay as-is
    db.DeleteFederationImport(importID)
    return fmt.Errorf("activation failed: %w", err)
}

// 3. Activate (only after both JWTs pushed successfully)
db.ActivateFederation(exportID, importID)
```

Revocation flow (transactional):
```go
// 1. Update DB (mark as revoked)
db.RevokeFederationExport(exportID)

// 2. Transactional push: both accounts rebuilt without claims
err := accounts.RebuildAndPushMultipleAccounts(db, []string{orgA, orgB}, operatorKP, sysConn)
if err != nil {
    // DB already marked revoked but JWTs may be stale
    // Log error — next RebuildAndPush will fix consistency
    slog.Error("revocation JWT push failed", "export", exportID, "err", err)
}
```

Inside `RebuildAndPushAccountJWT` (from nats-accounts), the export/import
claims are built from DB state:

```go
// In internal/accounts/jwt.go
func RebuildAndPushAccountJWT(db *sql.DB, orgID string, ...) error {
    org, _ := db.GetOrg(orgID)
    claims := jwt.NewAccountClaims(org.NatsPublicKey)
    claims.Name = org.Name
    applyLimitsFromTier(claims, org.BillingTier)

    // Federation exports: DB rows → JWT claims
    for _, exp := range db.ListActiveExports(orgID) {
        targetOrg, _ := db.GetOrg(exp.TargetOrg)
        claims.Exports.Add(&jwt.Export{
            Name:    exp.Name,
            Subject: jwt.Subject(exp.Subject),
            Type:    jwt.Stream,
        })
    }

    // Federation imports: DB rows → JWT claims
    for _, imp := range db.ListActiveImports(orgID) {
        export, _ := db.GetFederationExport(imp.ExportID)
        sourceOrg, _ := db.GetOrg(export.OrgID)
        claims.Imports.Add(&jwt.Import{
            Name:         export.Name,
            Subject:      jwt.Subject(export.Subject),
            Account:      sourceOrg.NatsPublicKey,
            LocalSubject: jwt.RenamingSubject(imp.LocalSubject),
            Type:         jwt.Stream,
        })
    }

    signed, _ := claims.Encode(operatorKP)
    _, err := sysConn.Request("$SYS.REQ.CLAIMS.UPDATE", []byte(signed), 5*time.Second)
    return err
}
```

This means:
- **DB is always source of truth** — JWT is a derived artifact
- **No "load current JWT" needed** — eliminates fetch-from-NATS round trip
- **No JWT↔DB desync risk** — JWT always reflects exact DB state
- **Idempotent** — rebuild + push can be called anytime as a consistency check

## Risks

| Risk | Severity | Mitigation |
|---|---|---|
| Dependency on nats-accounts | BLOCKER | Cannot ship until NATS Accounts exist. Designed and planned. |
| Subject collision on import | MEDIUM | Warn if target local_subject already has local publishers. `--as` flag encourages explicit remapping. |
| Export abuse (data exfiltration) | MEDIUM | Exports require org admin role. Audit log. Rate limits on creation. Dashboard shows all active exports. Subject validation rejects naked `>`. |
| JWT race condition | MEDIUM | Serialize RebuildAndPush per account (Go mutex in accounts package). DB write → rebuild → push is atomic per account. |
| Stale exports (org deleted) | LOW | `ON DELETE CASCADE` on FK. Org deletion revokes account JWT — NATS cuts all flows. Importing org's imports marked `terminated`, JWT rebuilt. |
| Pending export staleness | LOW | 7-day TTL. Pending exports auto-expire. Cron or on-access check. |

## File Structure

```
internal/federation/
  # Existing (kept for potential cross-cluster use):
  federation.go          # Bridge manager (interceptor-style bridges)
  client.go              # HTTP/WS client (dormant, not used by federation-cli)

  # New:
  exports.go             # Export CRUD + JWT claim management
  imports.go             # Import CRUD + JWT claim management
  matcher.go             # Detect export↔import matches, transactional activation

cmd/notif/
  federation.go          # CLI subcommands (export/import/ls/rm/accept)

internal/server/
  federation_handlers.go # HTTP handlers for federation API
```

## Dependencies

- **Blocked by**: `nats-accounts` (no accounts = no exports/imports)
- **Depends on**: `orgs` table (from nats-accounts)
- **Depends on**: `nats-io/jwt/v2` (same as nats-accounts, for JWT modification)
- **Depends on**: Postgres (federation metadata)

## Acceptance Criteria

- [ ] `notif federation export orders.> --to org-b` creates export (status: pending)
- [ ] Pending export emits `federation.export.created` notif event (async notification)
- [ ] `notif federation import orders.> --from org-a` creates import and activates flow
- [ ] Activation is transactional: both account JWTs pushed or neither (rollback on failure)
- [ ] Events published in org_a appear in org_b via NATS native export/import (zero-copy)
- [ ] `notif federation rm` revokes immediately (transactional JWT push → NATS hot reload)
- [ ] Bilateral consent: export without matching import does not flow
- [ ] Subject remapping: import `orders.>` as `vendor-orders.>` works
- [ ] Wildcard validation: only naked `>` rejected (single-level like `orders.>` is valid)
- [ ] Pending export TTL: auto-revokes after 7 days if no matching import
- [ ] Org deletion cascades: exports cascade-deleted, imports marked `terminated`, JWTs rebuilt
- [ ] `notif federation status <id>` shows consumer lag and flow health
- [ ] `notif federation ls --terminated` shows imports whose source org was deleted
- [ ] Dashboard shows active exports/imports with status
- [ ] JWT-as-derived-view: RebuildAndPush from DB state (never load/modify existing JWT)
- [ ] RebuildAndPush serialized per account via mutex (no race conditions)
- [ ] All federation operations audit logged
