# Wish: notif connect Phase 1 (Local-Only Bridge)

**Status:** DRAFT
**Slug:** `notif-connect-phase1`
**Created:** 2026-02-18

---

## Summary

Implement `notif connect` Phase 1: a local-only bridge that runs interceptors (jq transforms) on a local NATS server, managed as a system service (systemd/launchd). No cloud connectivity yet — Phase 2 (leaf node to notif.sh cloud) ships after `nats-accounts`. This phase is fully independent and can ship in parallel.

---

## Dependencies

- **depends-on:** nothing (Phase 1 is independent)
- **blocks:** nothing directly (Phase 2 will depend on `nats-accounts`)

---

## Scope

### IN
- Cobra subcommand tree: `notif connect install/start/stop/status/logs/uninstall/run`
- System service via `kardianos/service` (launchd on macOS, systemd on Linux)
- Bridge runtime: connect to local NATS, create NOTIF_BRIDGE stream, run interceptors
- JetStream stream handling with conflict detection
- Config resolution: CLI flags → env vars → `~/.notif/connect.yaml`
- Status reporting: human-readable (default) + `--json`, heartbeat 10s, stale 30s, dead 60s
- Config drift detection on heartbeat
- Log rotation via lumberjack (50MB, 3 backups, 14 days)
- `notif connect run` foreground mode for dev/debug

### OUT
- `--cloud` flag (Phase 2, after nats-accounts)
- Credential provisioning (Phase 2)
- Leaf node configuration (Phase 2)
- Managed/embedded NATS mode (removed from design)
- Dashboard or web UI for bridge status
- Multi-bridge coordination (one bridge per host)

---

## Decisions

- **DEC-1:** `kardianos/service` for cross-platform service management. Abstracts launchd/systemd. Well-maintained Go library.
- **DEC-2:** Human-readable status by default, `--json` for scripts. Users interact with status visually, scripts parse JSON.
- **DEC-3:** Heartbeat 10s, stale 30s, dead 60s. Balanced between freshness and resource usage. Not 2s (excessive) or 30s (too slow to detect issues).
- **DEC-4:** Config persisted to `~/.notif/connect.yaml` on install. Service reads config from file on boot, not from flags.
- **DEC-5:** Stream conflict detection on install. Fail with clear error if existing stream captures same subjects. Offer `--stream` to reuse.
- **DEC-6:** Log rotation via lumberjack. 50MB max, 3 backups, 14 days retention, compressed. No manual log management.
- **DEC-7:** Config drift detection. Bridge compares running config vs file on each heartbeat. Warns if mismatch detected.

---

## Success Criteria

- [ ] `notif connect install --nats nats://localhost:4222` registers system service
- [ ] `notif connect start` → service runs, connects to local NATS
- [ ] `notif connect status` shows human-readable output (bridge state, NATS connection, interceptors, throughput)
- [ ] `notif connect status --json` outputs machine-readable JSON
- [ ] Heartbeat: 10s interval, stale warning at 30s, dead at 60s
- [ ] Config drift: warns if `~/.notif/connect.yaml` differs from running config
- [ ] Interceptors from `--interceptors` YAML run with MaxAckPending: 1000
- [ ] `notif connect stop && notif connect uninstall` cleanly removes service
- [ ] Works on macOS (launchd) and Linux (systemd)
- [ ] Survives reboot (service auto-starts)
- [ ] `notif connect run` works in foreground
- [ ] Stream conflict detection warns on subject overlap
- [ ] Log rotation: 50MB, 3 backups, 14 days

---

## Assumptions

- **ASM-1:** User has NATS running locally (this bridge doesn't manage NATS)
- **ASM-2:** `notif` CLI already exists as a cobra app with `internal/interceptor` package
- **ASM-3:** JetStream is enabled on the local NATS server
- **ASM-4:** User has write access to `~/.notif/` directory

## Risks

- **RISK-1:** `kardianos/service` quirks on different OS versions. — Mitigation: test on macOS 14+ and Ubuntu 22.04+. Foreground mode (`run`) as fallback.
- **RISK-2:** Stream conflict detection false positives with overlapping wildcards. — Mitigation: implement proper `subjectsOverlap()` using NATS subject matching rules, not string comparison.
- **RISK-3:** Status file stale if service crashes without cleanup. — Mitigation: PID check — if PID in status file is dead, show "crashed" regardless of heartbeat.

---

## Execution Groups

### Group A: Service Scaffold + Lifecycle

**Goal:** `notif connect install/start/stop/uninstall` manage a system service.

**Deliverables:**
- `cmd/notif/connect.go` — cobra subcommand tree (install, start, stop, status, logs, uninstall, run)
- `internal/bridge/service.go` — kardianos/service integration (ServiceProgram interface)
- Config file: `~/.notif/connect.yaml` written on install, read on boot

**Acceptance Criteria:**
- [ ] `notif connect install --nats nats://localhost:4222` writes config and registers service
- [ ] `notif connect start` starts the service (launchd or systemd)
- [ ] `notif connect stop` stops it cleanly
- [ ] `notif connect uninstall` removes service registration and config
- [ ] Service auto-starts on reboot
- [ ] Works on macOS and Linux

**Validation:** `notif connect install --nats nats://localhost:4222 && notif connect start && sleep 2 && notif connect status && notif connect stop && notif connect uninstall`

---

### Group B: Bridge Runtime + Interceptors

**Goal:** Bridge connects to local NATS, creates stream, runs interceptors.

**Deliverables:**
- `internal/bridge/bridge.go` — Bridge struct (Start, Stop, Status)
- JetStream stream creation (NOTIF_BRIDGE) with conflict detection
- Interceptor manager integration (from existing `internal/interceptor`)
- Consumer config with MaxAckPending: 1000 backpressure
- `notif connect run` foreground mode

**Acceptance Criteria:**
- [ ] Bridge connects to local NATS and creates NOTIF_BRIDGE stream
- [ ] Interceptors from YAML config process events correctly
- [ ] MaxAckPending: 1000 backpressure enforced
- [ ] Stream conflict detection: error if topics overlap existing stream
- [ ] `--stream <name>` reuses existing stream
- [ ] `notif connect run` runs in foreground until Ctrl+C

**Validation:** `notif connect run --nats nats://localhost:4222 --topics "test.>" &` then `nats pub test.hello '{"data":1}'`

---

### Group C: Config Resolution

**Goal:** Config priority: CLI flags → env vars → config file.

**Deliverables:**
- `internal/bridge/config.go` — Config struct with resolution logic
- Flag parsing: `--nats`, `--topics`, `--interceptors`, `--cloud` (placeholder)
- Env vars: `NATS_URL`, `NOTIF_API_KEY`, `NOTIF_CLOUD_URL`
- Config file: `~/.notif/connect.yaml` (auto-generated by install)
- Config drift detection (compare running vs file on heartbeat)

**Acceptance Criteria:**
- [ ] CLI flags override env vars override config file
- [ ] `notif connect install` persists all resolved config to `~/.notif/connect.yaml`
- [ ] Service reads config from file on boot (no flags needed)
- [ ] Config drift: detected and reported in status output

**Validation:** `notif connect install --nats nats://localhost:4222 --topics "a.>" && cat ~/.notif/connect.yaml`

---

### Group D: Status + Heartbeat + Logging

**Goal:** Status reporting with heartbeat, staleness detection, and log rotation.

**Deliverables:**
- `internal/bridge/status.go` — StatusReporter (heartbeat writer, staleness detection)
- Status file: `~/.notif/connect.status.json` (written every 10s)
- `notif connect status` — human-readable (default) + `--json`
- `notif connect logs` — tail `~/.notif/connect.log`
- Log rotation via lumberjack
- PID-based crash detection

**Acceptance Criteria:**
- [ ] Heartbeat written every 10s to status file
- [ ] `notif connect status` shows human-readable output
- [ ] `notif connect status --json` outputs JSON
- [ ] Stale warning at 30s, dead at 60s
- [ ] PID check: if PID dead, show "crashed" even if heartbeat recent
- [ ] Config drift shown in status output if detected
- [ ] `notif connect logs` tails the log file
- [ ] Log file rotates at 50MB, keeps 3 backups, 14 days max, compressed

**Validation:** `notif connect run --nats nats://localhost:4222 &` then `sleep 15 && notif connect status`

---

## Review Results

_Populated by `/review` after execution completes._

---

## Files to Create/Modify

```
# New
cmd/notif/connect.go                 # Cobra subcommands
internal/bridge/bridge.go            # Bridge struct (runtime)
internal/bridge/service.go           # kardianos/service integration
internal/bridge/config.go            # Config resolution + drift detection
internal/bridge/status.go            # Status reporting + heartbeat
internal/bridge/leafnode.go          # Leaf node config (Phase 2 placeholder)
internal/bridge/provision.go         # Credential provisioning (Phase 2 placeholder)

# Modified
cmd/notif/root.go                    # Register connect subcommand
go.mod                               # Add kardianos/service, lumberjack

# Reused as-is
internal/interceptor/                # Existing interceptor package
```
