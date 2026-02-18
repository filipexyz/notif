# nats-accounts

## Problem
notif.sh cloud hoje usa um único NATS account com isolamento a nível de aplicação (subjects prefixados com `org_id.project_id` no código Go). Isso impede federação real entre tenants, não oferece isolamento de protocolo, e não permite billing granular por uso de recursos NATS.

## Context
- notif.sh cloud today: single NATS account, tenants isolated at application level
- NATS Accounts = protocol-level isolation (subjects, streams, KV per account)
- Prerequisite for: federation between tenants, notif:// protocol, cloud multi-tenancy
- NATS has nsc CLI for managing operators/accounts/users — need to abstract it
- Zero existing tenants (greenfield)

## Scope

**IN:**
- NATS Account per tenant (1 org = 1 NATS account)
- Operator/Account/User JWT model via `nsc`
- `nats-resolver` full type (dynamic, supports `nsc push`)
- System account for notifd (control plane)
- Subject prefixing within each account: `events.{org_id}.{project_id}.{topic}`
- Per-account NATS connections for self-hosted leaf nodes
- nsc operations abstracted by notif API/CLI (user never touches nsc)
- Account-level resource limits (connections, data, msg size) for billing

**OUT:**
- Multi-cluster NATS (single cluster for now)
- Custom auth providers (JWTs only, via nsc)
- Leaf node auto-provisioning (separate effort: `notif connect`)
- Federation between accounts (separate effort: `federation-cli`)

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Resolver type | `nats-resolver` full | Dynamic (hot reload), supports `nsc push`, stores JWTs in NATS itself. Memory resolver is static (requires server restart). URL resolver is legacy. |
| Migration strategy | Big bang | Zero existing tenants. No backward compatibility needed. |
| Control plane | System account for notifd | notifd connects as system account → full visibility. Subject prefixing (`org.project.topic`) for pragmatic isolation within account. Avoids per-tenant connections in notifd. |
| Subject model | Prefixed within account | `events.{org_id}.{project_id}.{topic}` — same as today but now within a NATS account boundary. Protocol-level isolation + application-level prefixing = defense in depth. |
| Leaf node strategy | Per-account connections | Self-hosted NATS (via `notif connect`) gets its own account + NKey credentials. Connects as leaf node to cloud with account-scoped subjects only. |
| nsc abstraction | notif wraps nsc | User-facing operations (create org, add user, rotate keys) go through notif API/CLI. nsc is an implementation detail. |
| Blast radius mitigation | mTLS + NKey + co-located | System account is powerful but: (1) mTLS between notifd↔NATS, (2) NKey auth (no passwords), (3) notifd + NATS run on same host/pod. No network exposure of system creds. |

## Risks

| Risk | Severity | Mitigation |
|---|---|---|
| System account blast radius | HIGH | mTLS + NKey auth. notifd and NATS co-located (same pod/host). System creds never leave the machine. Audit log on system account operations. |
| nsc complexity leaking to users | MEDIUM | Full abstraction in notif API. Users create orgs/projects → notif handles account/user JWTs behind the scenes. |
| Resolver state loss | MEDIUM | nats-resolver full stores JWTs in NATS JetStream (replicated). Backup via `nsc pull`. |
| Account limit misconfiguration | LOW | Defaults from billing tier. Operators (us) set limits, not tenants. Validation on limit changes. |
| Subject prefix collision | LOW | Impossible — each account is protocol-isolated. Prefix is defense-in-depth, not primary isolation. |

## Criteria
- [ ] notifd boots with operator + system account configured via nats-resolver full
- [ ] `notif org create <name>` provisions a NATS account + signing key via nsc (abstracted)
- [ ] `notif org delete <name>` removes the NATS account and revokes all user JWTs
- [ ] Tenant A cannot subscribe to Tenant B's subjects (protocol-level isolation verified)
- [ ] System account can observe all accounts (for admin/metrics dashboard)
- [ ] Account-level resource limits (max connections, max data) configurable per billing tier
- [ ] Self-hosted leaf node connects with account-scoped NKey credentials
- [ ] Zero-downtime: adding/removing accounts does not restart NATS server
