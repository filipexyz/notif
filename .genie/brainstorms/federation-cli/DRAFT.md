# federation-cli

## Problem
Quando NATS Accounts existirem no cloud, tenants precisam de uma forma de compartilhar
subjects entre si (ex: Org A exporta `orders.>` para Org B). Hoje a federation é
application-level (WS+HTTP entre instâncias notif separadas). A federation-cli traz
federação nativa NATS (export/import entre accounts no mesmo cluster) com UX simples.

## Scope

**IN:**
- `notif federation export` — compartilhar subject de um account para outro
- `notif federation import` — receber subject compartilhado de outro account
- `notif federation ls` — listar exports/imports ativos
- `notif federation rm` — remover um export/import
- Opera no cloud (account ↔ account no mesmo cluster NATS)
- Consent model: ambas as partes devem aceitar (export + import)
- Subject remapping opcional (ex: importar `orders.>` como `vendor-orders.>`)

**OUT:**
- Federação local↔local (sem cloud)
- Bridge local↔cloud (`notif connect` faz isso)
- Federation HTTP/WS existente (permanece para cross-instance, cross-cluster)
- Service exports (req/reply) — apenas stream exports (pub/sub) por agora

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Mechanism | NATS native exports/imports | Zero-copy dentro do cluster. Protocol-level, não application-level. NATS resolve delivery, backpressure, e ordering. |
| Export type | Stream (pub/sub) | Eventos são pub/sub. Service exports (req/reply) é stretch goal. |
| Authorization | Bilateral consent | Org A cria export, Org B cria import. Ambos precisam existir para o fluxo ativar. Previne leak unilateral. |
| Subject mapping | Opcional, na importação | Importer decide como remapear. Default: mesmo subject name. Ex: importar `orders.>` como `partner.orders.>`. |
| Managed by | notifd (system account) | System account tem visibility total. notifd aplica export/import via `nsc` + `nsc push`. Tenants não tocam nsc. |
| State storage | Postgres (notif cloud DB) | Metadata de federation (who, what, when, status) vive no DB. NATS JWTs são o enforcement, DB é o source of truth para UI/API. |
| Revocation | Immediate | Remover export/import → `nsc` revoga → `nsc push` → NATS corta o fluxo imediatamente (hot reload). |

## Risks

| Risk | Severity | Mitigation |
|---|---|---|
| Dependency on nats-accounts | BLOCKER | Cannot ship until NATS Accounts exist in cloud. Planned and designed. |
| Subject collision on import | MEDIUM | Require explicit subject mapping on import. Warn if target subject already has local publishers. |
| Export abuse (data exfiltration) | MEDIUM | Exports require org admin role. Audit log. Rate limits on export creation. Dashboard shows all active exports. |
| Stale exports (org deleted) | LOW | Cascade: org deletion revokes all exports/imports. nsc handles JWT revocation. |
| NATS export/import complexity | LOW | Fully abstracted by CLI/API. User says "share orders with Org B", notif handles nsc commands. |

## User Journey

```bash
# Org A: exportar orders para Org B
notif federation export orders.> --to org-b --name "order-feed"
# → Export created (pending import from org-b)

# Org B: importar orders de Org A
notif federation import orders.> --from org-a --as vendor-orders.>
# → Import active! Events flowing.

# List
notif federation ls
# → order-feed  export  orders.>  → org-b  active
# → vendor-feed import  orders.>  ← org-a  active (as vendor-orders.>)

# Remove
notif federation rm order-feed
# → Export revoked. Flow stopped.
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
│  │  Manages exports/imports via nsc      │       │
│  │  Stores metadata in Postgres          │       │
│  └──────────────────────────────────────┘       │
└─────────────────────────────────────────────────┘
```

## Implementation Plan

### 1. DB Schema

```sql
CREATE TABLE federation_exports (
  id          UUID PRIMARY KEY,
  org_id      UUID NOT NULL REFERENCES orgs(id),
  name        TEXT NOT NULL,
  subject     TEXT NOT NULL,           -- exported subject pattern
  target_org  UUID NOT NULL,           -- who can import
  status      TEXT DEFAULT 'pending',  -- pending, active, revoked
  created_at  TIMESTAMPTZ DEFAULT now(),
  revoked_at  TIMESTAMPTZ
);

CREATE TABLE federation_imports (
  id          UUID PRIMARY KEY,
  org_id      UUID NOT NULL REFERENCES orgs(id),
  export_id   UUID NOT NULL REFERENCES federation_exports(id),
  local_subject TEXT,                  -- remapped subject (nullable = same)
  status      TEXT DEFAULT 'pending',  -- pending, active, revoked
  created_at  TIMESTAMPTZ DEFAULT now(),
  revoked_at  TIMESTAMPTZ
);
```

### 2. API Endpoints

```
POST   /api/v1/federation/exports       # Create export
GET    /api/v1/federation/exports       # List exports
DELETE /api/v1/federation/exports/:id   # Revoke export

POST   /api/v1/federation/imports       # Create import
GET    /api/v1/federation/imports       # List imports
DELETE /api/v1/federation/imports/:id   # Revoke import
```

### 3. CLI Commands (notif CLI)

```bash
notif federation export <subject> --to <org> --name <name>
notif federation import <subject> --from <org> [--as <local-subject>]
notif federation ls [--exports | --imports]
notif federation rm <name-or-id>
```

### 4. nsc Integration (in notifd)

On export create:
```bash
nsc edit account org_a --allow-export "orders.>" --account org_b
nsc push -a org_a
```

On import create:
```bash
nsc edit account org_b --allow-import --from org_a --subject "orders.>" --local "vendor-orders.>"
nsc push -a org_b
```

On revoke:
```bash
nsc edit account org_a --rm-export "orders.>"
nsc push -a org_a
```

### 5. Activation Flow

```
1. Org A creates export (status: pending)
2. Org B creates matching import (status: pending)
3. notifd detects match → applies nsc export+import → nsc push
4. Both statuses → active
5. Events flow via NATS native export/import (zero-copy)
```

## File Structure

```
internal/federation/
  # Existing (keep as-is):
  federation.go          # HTTP/WS bridge (cross-instance)
  client.go              # WS subscribe + HTTP emit

  # New:
  accounts.go            # NATS account-level federation
  exports.go             # Export management + nsc integration
  imports.go             # Import management + nsc integration

cmd/notif/
  federation.go          # CLI subcommands (export/import/ls/rm)
```

## Dependency

**Blocked by:** `nats-accounts` (sem accounts, sem isolamento, sem exports/imports)

## Criteria

- [ ] `notif federation export orders.> --to org-b` cria export (status: pending)
- [ ] `notif federation import orders.> --from org-a` cria import e ativa o fluxo
- [ ] Events publicados em org_a.orders.> aparecem em org_b.vendor-orders.> (NATS native)
- [ ] `notif federation rm` revoga imediatamente (NATS hot reload, zero delay)
- [ ] Bilateral consent: export sem import correspondente não flui
- [ ] Subject remapping funciona: importar `orders.>` como `vendor-orders.>`
- [ ] Dashboard mostra exports/imports ativos com throughput
- [ ] Org deletion cascata: revoga todos os exports/imports da org
