# Agent Swarm Protocol

> Devin-style AI task orchestration over notif.sh pub/sub — but with real-time events instead of polling.

## Inspiration: Devin API

Devin uses REST API with polling. We can do better with pub/sub:

| Devin (Polling) | Agent Swarm (Events) |
|-----------------|---------------------|
| `POST /sessions` | `agents.session.create` |
| `GET /sessions/{id}` (poll) | `agents.session.*` (subscribe once) |
| Playbooks | Playbooks (same concept) |
| Snapshots | Worktrees (git-native) |
| Structured Output | Structured Output (same) |

**Key difference:** Instead of polling for status, clients subscribe to session events and get real-time updates.

## Vision

Fire multiple tasks at Claude agents that automatically create isolated git worktrees, work in parallel, and produce PRs.

```
┌─────────────────────────────────────────────────────────────────┐
│  Orchestrator                                                   │
│                                                                 │
│  "Add user authentication"  ──┬──►  Worker 1 (worktree: auth)   │
│  "Fix pagination bug"         ├──►  Worker 2 (worktree: fix-pg) │
│  "Add dark mode"              └──►  Worker 3 (worktree: dark)   │
│                                                                 │
│  All working in parallel, isolated branches, auto-PR on done    │
└─────────────────────────────────────────────────────────────────┘
```

## Why Worktrees?

- **Parallel work** - Multiple agents work on different features simultaneously
- **Clean isolation** - Each agent has its own directory, no conflicts
- **Branch per task** - Easy to track, review, merge
- **Automatic cleanup** - Remove worktree after PR merged
- **Same repo, multiple checkouts** - Efficient disk usage

## Protocol

### Topics

The protocol is executor-agnostic. Claude Code is one executor, but could be Codex, Gemini, or custom.

Topics are scoped per agent: `agents.<agent-name>.*`. Notif API keys provide tenant isolation.

**Discovery (broadcast):**
| Topic | Direction | Purpose |
|-------|-----------|---------|
| `agents.discover` | Client → Workers | Find available agents (broadcast) |
| `agents.available` | Workers → Client | Announce with capabilities |

Discovery uses timeout-based collection: client subscribes to `agents.available`, emits `agents.discover`, collects responses within timeout (e.g., 2s).

**Session Lifecycle (per-agent):**
| Topic | Direction | Purpose |
|-------|-----------|---------|
| `agents.<name>.session.create` | Client → Worker | Create session (with worktree) |
| `agents.<name>.session.message` | Client → Worker | Follow-up prompt OR human input for blocked |
| `agents.<name>.session.cancel` | Client → Worker | Cancel running session |
| `agents.<name>.session.started` | Worker → Client | Session accepted, worktree ready |
| `agents.<name>.session.output` | Worker → Client | Structured output update |
| `agents.<name>.session.completed` | Worker → Client | Done, PR URL included |
| `agents.<name>.session.failed` | Worker → Client | Error details (no auto-retry) |
| `agents.<name>.session.blocked` | Worker → Client | Needs human input |

**Session Continuity:** Worker manages Claude session ID internally. On `session.message`, worker auto-resumes the Claude session.

**Concurrency:** Workers handle multiple sessions in parallel (separate goroutines).

**Worktree Management (per-agent):**
| Topic | Direction | Purpose |
|-------|-----------|---------|
| `agents.<name>.worktree.cleanup` | Client → Worker | Remove worktree |
| `agents.<name>.worktree.list` | Client → Worker | List active worktrees |

### Discovery Message

```json
{
  "agent": "coder-1",
  "description": "Full-stack TypeScript agent",
  "hostname": "macbook-pro.local",
  "tags": ["typescript", "react", "node"],

  "executor": {
    "type": "claude",              // claude | codex | gemini | custom
    "version": "claude-sonnet-4-20250514",
    "cli": "claude"                // command to invoke
  },

  "git": {
    "repo": "filipexyz/myapp",
    "branch": "main",
    "status": "clean",
    "remote_url": "git@github.com:filipexyz/myapp.git"
  },

  "project": {
    "type": "node",
    "name": "myapp",
    "path": "/Users/dev/myapp"
  },

  "worktree": {
    "enabled": true,
    "base_path": "/Users/dev/worktrees/myapp",
    "active": [
      {"branch": "feat/auth", "path": "/Users/dev/worktrees/myapp/feat-auth", "task_id": "task-123"}
    ]
  },

  "capabilities": {
    "tools": ["Read", "Edit", "Bash", "WebSearch"],
    "mcp_servers": ["context7"]
  },

  "resources": {
    "budget_usd": 5.0,
    "budget_remaining_usd": 4.50,
    "status": "idle"
  }
}
```

### Task Assignment

```json
{
  "task_id": "task-abc123",
  "prompt": "Add user authentication with JWT and refresh tokens",

  "worktree": {
    "enabled": true,
    "branch": "feat/auth",
    "base_branch": "main"
  },

  "pr": {
    "auto_create": true,
    "draft": true,
    "title": "Add user authentication",
    "labels": ["enhancement", "auth"]
  },

  "options": {
    "budget_usd": 2.0,
    "timeout_seconds": 600
  }
}
```

### Task Started

```json
{
  "task_id": "task-abc123",
  "agent": "coder-1",
  "started_at": "2025-01-03T10:00:00Z",

  "worktree": {
    "path": "/Users/dev/worktrees/myapp/feat-auth",
    "branch": "feat/auth"
  }
}
```

### Task Progress (Optional Streaming)

```json
{
  "task_id": "task-abc123",
  "timestamp": "2025-01-03T10:01:30Z",
  "message": "Created auth middleware, now working on JWT validation..."
}
```

### Task Completed

```json
{
  "task_id": "task-abc123",
  "completed_at": "2025-01-03T10:05:00Z",

  "result": {
    "summary": "Implemented JWT authentication with refresh tokens...",
    "branch": "feat/auth",
    "commits": 3,
    "files_changed": 8
  },

  "pr": {
    "url": "https://github.com/filipexyz/myapp/pull/42",
    "number": 42,
    "draft": true
  },

  "cost_usd": 0.45,
  "duration_seconds": 300
}
```

### Task Failed

```json
{
  "task_id": "task-abc123",
  "failed_at": "2025-01-03T10:03:00Z",

  "error": {
    "message": "Could not resolve peer dependency",
    "type": "dependency_error"
  },

  "worktree": {
    "preserved": true,
    "path": "/Users/dev/worktrees/myapp/feat-auth"
  },

  "cost_usd": 0.12
}
```

## Worker Implementation (Go)

```go
// Executor interface - Claude, Codex, Gemini, etc.
type Executor interface {
    Run(ctx context.Context, req ExecutorRequest) (<-chan ExecutorEvent, error)
}

type ExecutorRequest struct {
    Prompt           string
    Cwd              string
    Budget           *float64
    StructuredOutput json.RawMessage
}

type ExecutorEvent struct {
    Type             string          // "output", "completed", "error"
    StructuredOutput json.RawMessage
    Result           string
    Cost             float64
    Error            error
}

// Worker handles sessions
type Worker struct {
    client   *notif.Client
    executor Executor
    name     string
    git      *GitInfo
    worktreeBase string
}

func (w *Worker) HandleSession(ctx context.Context, session *SessionCreate) {
    var worktreePath, branch string

    // 1. Create worktree if requested
    if session.Worktree.Enabled {
        branch = session.Worktree.Branch
        if branch == "" {
            branch = slugify(session.Prompt)
        }
        worktreePath = filepath.Join(w.worktreeBase, branch)

        if err := w.createWorktree(ctx, worktreePath, branch, session.Worktree.BaseBranch); err != nil {
            w.emitFailed(session.SessionID, err, worktreePath)
            return
        }

        w.client.Emit(ctx, "agents.session.started", map[string]any{
            "session_id": session.SessionID,
            "agent":      w.name,
            "worktree":   map[string]string{"path": worktreePath, "branch": branch},
        })
    }

    // 2. Run executor
    cwd := worktreePath
    if cwd == "" {
        cwd = w.git.Path
    }

    events, err := w.executor.Run(ctx, ExecutorRequest{
        Prompt:           session.Prompt,
        Cwd:              cwd,
        Budget:           session.Options.Budget,
        StructuredOutput: session.StructuredOutput,
    })
    if err != nil {
        w.emitFailed(session.SessionID, err, worktreePath)
        return
    }

    // 3. Stream events
    var result ExecutorEvent
    for event := range events {
        switch event.Type {
        case "output":
            w.client.Emit(ctx, "agents.session.output", map[string]any{
                "session_id":        session.SessionID,
                "structured_output": event.StructuredOutput,
            })
        case "completed":
            result = event
        case "error":
            w.emitFailed(session.SessionID, event.Error, worktreePath)
            return
        }
    }

    // 4. Create PR if requested
    var prURL string
    if session.Options.AutoPR {
        prURL, _ = w.createPR(ctx, cwd, session.Prompt)
    }

    // 5. Emit completion
    w.client.Emit(ctx, "agents.session.completed", map[string]any{
        "session_id":        session.SessionID,
        "result":            result.Result,
        "structured_output": result.StructuredOutput,
        "pr":                map[string]string{"url": prURL},
        "cost_usd":          result.Cost,
    })
}

// Claude executor implementation
type ClaudeExecutor struct{}

func (e *ClaudeExecutor) Run(ctx context.Context, req ExecutorRequest) (<-chan ExecutorEvent, error) {
    args := []string{"-p", "--output-format", "stream-json"}
    if req.Budget != nil {
        args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", *req.Budget))
    }
    if req.StructuredOutput != nil {
        args = append(args, "--json-schema", string(req.StructuredOutput))
    }

    cmd := exec.CommandContext(ctx, "claude", args...)
    cmd.Dir = req.Cwd
    cmd.Stdin = strings.NewReader(req.Prompt)

    stdout, _ := cmd.StdoutPipe()
    cmd.Start()

    events := make(chan ExecutorEvent)
    go func() {
        defer close(events)
        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            var msg map[string]any
            json.Unmarshal(scanner.Bytes(), &msg)

            if output, ok := msg["structured_output"]; ok {
                events <- ExecutorEvent{Type: "output", StructuredOutput: output.(json.RawMessage)}
            }
            if result, ok := msg["result"]; ok {
                events <- ExecutorEvent{
                    Type:   "completed",
                    Result: result.(string),
                    Cost:   msg["total_cost_usd"].(float64),
                }
            }
        }
        cmd.Wait()
    }()

    return events, nil
}
```

## Orchestrator Implementation (Go)

```go
type Orchestrator struct {
    client *notif.Client
}

func (o *Orchestrator) Dispatch(ctx context.Context, prompts []string, repo string) (map[string]*SessionResult, error) {
    // 1. Discover available agents for this repo
    agents, err := o.discover(ctx, repo)
    if err != nil {
        return nil, err
    }
    if len(agents) == 0 {
        return nil, fmt.Errorf("no agents available for %s", repo)
    }

    // 2. Create sessions, round-robin across agents
    pending := make(map[string]string) // session_id -> prompt
    for i, prompt := range prompts {
        agent := agents[i%len(agents)]
        sessionID := fmt.Sprintf("sess-%s", randomHex(8))

        o.client.Emit(ctx, "agents.session.create", map[string]any{
            "session_id": sessionID,
            "agent":      agent.Name,
            "prompt":     prompt,
            "worktree":   map[string]bool{"enabled": true},
            "options":    map[string]bool{"auto_pr": true},
        })

        pending[sessionID] = prompt
    }

    // 3. Subscribe and stream progress
    results := make(map[string]*SessionResult)
    sub := o.client.Subscribe("agents.session.*")

    for event := range sub.Events() {
        sessionID, _ := event.Data["session_id"].(string)
        if _, ok := pending[sessionID]; !ok {
            continue
        }

        switch event.Topic {
        case "agents.session.output":
            // Real-time progress!
            fmt.Printf("[%s] %v\n", sessionID, event.Data["structured_output"])

        case "agents.session.completed":
            results[sessionID] = &SessionResult{
                Result: event.Data["result"].(string),
                PR:     event.Data["pr"].(map[string]any)["url"].(string),
                Cost:   event.Data["cost_usd"].(float64),
            }
            delete(pending, sessionID)

        case "agents.session.failed":
            results[sessionID] = &SessionResult{
                Error: event.Data["error"].(map[string]any)["message"].(string),
            }
            delete(pending, sessionID)
        }

        if len(pending) == 0 {
            break
        }
    }

    return results, nil
}
```

## CLI Usage

```bash
# Build
go build -o bin/ ./cmd/...

# Start a worker
./bin/worker --name coder-1 \
  --repo /path/to/myapp \
  --worktree-base /path/to/worktrees \
  --executor claude

# Dispatch sessions
./bin/swarm dispatch \
  --repo filipexyz/myapp \
  "Add user authentication" \
  "Fix pagination bug" \
  "Add dark mode toggle"

# Watch progress (real-time)
./bin/swarm watch

# List active sessions
./bin/swarm sessions

# Cleanup completed worktrees
./bin/swarm cleanup --merged-only
```

## Key Concepts

### Sessions

Sessions provide continuity across multiple prompts. Worker manages Claude session ID internally for auto-resume.

```json
// agents.<name>.session.create
{
  "session_id": "sess_abc123",        // client-generated for idempotency
  "prompt": "Set up the project and fix issue #42",

  "worktree": {
    "enabled": true,
    "branch": "fix/issue-42",
    "base_branch": "main"
  },

  "structured_output": {              // optional: JSON schema for live updates
    "type": "object",
    "properties": {
      "status": {"type": "string"},
      "files_changed": {"type": "array"},
      "tests_passing": {"type": "boolean"}
    }
  },

  "options": {
    "budget_usd": 2.0,
    "auto_pr": true
  }
}

// agents.<name>.session.message (follow-up OR human input for blocked)
{
  "session_id": "sess_abc123",
  "prompt": "Also add unit tests for the fix"
}
```

### Structured Output (Real-time)

Agent updates structured output as it works. Client sees live progress:

```json
// agents.session.output (emitted by worker as it progresses)
{
  "session_id": "sess_abc123",
  "structured_output": {
    "status": "investigating",
    "files_changed": ["src/auth.ts"],
    "tests_passing": null
  }
}

// Later update
{
  "session_id": "sess_abc123",
  "structured_output": {
    "status": "testing",
    "files_changed": ["src/auth.ts", "src/auth.test.ts"],
    "tests_passing": true,
    "root_cause": "JWT expiry not checked before refresh"
  }
}
```

### Worktrees as Snapshots

Devin uses VM snapshots. We use git worktrees (lighter, git-native):

| Devin Snapshot | Our Worktree |
|----------------|--------------|
| Full VM state | Git branch + directory |
| Heavy (GB) | Light (just files) |
| Slow to create | Instant (`git worktree add`) |
| Isolated environment | Isolated code, shared tools |

For full environment isolation, combine with:
- Docker containers per worktree
- nix-shell per worktree
- Virtual environments (Python venv, node_modules)

## Decisions Made

| Question | Decision |
|----------|----------|
| Agent targeting | Per-agent topics: `agents.<name>.session.*` |
| Playbooks | Skip for v1 |
| Concurrency | Parallel sessions (goroutines) |
| Namespacing | None (notif API key isolates) |
| Discovery correlation | Timeout-based collection |
| Blocked handling | Reuse `session.message` for human input |
| Session resume | Worker manages Claude session ID internally |
| Retry on failure | No auto-retry, client decides |

## Open Questions (v2)

1. **Worktree lifecycle** - When to clean up?
   - After PR merged?
   - Manual cleanup only?
   - TTL-based?

2. **Conflict handling** - What if two sessions touch same files?
   - Let PRs conflict and human resolves?
   - Detect overlap and warn?

3. **Multi-repo** - One agent, multiple repos?
   - Separate worktree bases per repo?
   - Dynamic repo cloning?

4. **Agent load balancing** - Multiple workers with same name?
   - Consumer groups for scaling?
   - Health checks?

## Complete Flow Example

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Client                                Worker (name: "coder")                 │
│                                                                              │
│ 1. Discover available agents                                                 │
│    ─────────────────────────────────────────────────────────────────────►    │
│    subscribe("agents.available")                                             │
│    emit("agents.discover", {})                                               │
│    ◄───────────────────────────────────────────────────────────────────      │
│    {agent: "coder", git: {...}, worktree: {...}}                             │
│                                                                              │
│ 2. Subscribe to agent's session events                                       │
│    ─────────────────────────────────────────────────────────────────────►    │
│    subscribe("agents.coder.session.*")                                       │
│                                                                              │
│ 3. Create session with worktree                                              │
│    ─────────────────────────────────────────────────────────────────────►    │
│    emit("agents.coder.session.create", {                                     │
│      session_id: "sess_abc",                                                 │
│      prompt: "Fix auth bug #42",                                             │
│      worktree: {enabled: true, branch: "fix/auth-42"}                        │
│    })                                                                        │
│                                                                              │
│ 4. Worker creates worktree, starts Claude                                    │
│    ◄─────────────────────────────────────────────────────────────────────    │
│    {topic: "agents.coder.session.started", worktree: "/path/fix-auth-42"}    │
│                                                                              │
│ 5. Structured output updates (real-time!)                                    │
│    ◄─────────────────────────────────────────────────────────────────────    │
│    {topic: "agents.coder.session.output", status: "investigating"}           │
│    ◄─────────────────────────────────────────────────────────────────────    │
│    {topic: "agents.coder.session.output", status: "writing_fix"}             │
│                                                                              │
│ 6. Session completes with PR                                                 │
│    ◄─────────────────────────────────────────────────────────────────────    │
│    {topic: "agents.coder.session.completed",                                 │
│     pr: {url: "github.com/.../pull/15"},                                     │
│     result: "Fixed JWT refresh bug..."}                                      │
│                                                                              │
│ 7. (Optional) Follow-up in same session (worker auto-resumes Claude)        │
│    ─────────────────────────────────────────────────────────────────────►    │
│    emit("agents.coder.session.message", {                                    │
│      session_id: "sess_abc",                                                 │
│      prompt: "Add more edge case tests"                                      │
│    })                                                                        │
│                                                                              │
│ 8. Cleanup worktree after PR merged                                          │
│    ─────────────────────────────────────────────────────────────────────►    │
│    emit("agents.coder.worktree.cleanup", {session_id: "sess_abc"})           │
└──────────────────────────────────────────────────────────────────────────────┘
```

## Comparison: Devin API vs Agent Swarm

| Feature | Devin API | Agent Swarm |
|---------|-----------|-------------|
| **Protocol** | REST + Polling | Pub/Sub Events |
| **Progress** | Poll every N seconds | Real-time stream |
| **Multi-agent** | Separate sessions | Discover + route by name |
| **Isolation** | VM Snapshots (heavy) | Git Worktrees (instant) |
| **Executor** | Devin only | Claude, Codex, Gemini, etc. |
| **Cost** | $$$ (VM compute) | $ (just LLM API) |
| **Self-hosted** | No | Yes (notif.sh) |
| **Session resume** | Yes | Yes (worker-managed) |

## Future Ideas

- **Agent Pool** - Pre-created worktrees ready to accept tasks
- **Dependency Graph** - Task B waits for Task A's branch
- **Auto-merge** - Merge PRs that pass CI automatically
- **Cost Budgets** - Per-task and per-agent spending limits
- **Priority Queue** - Urgent tasks jump the line
- **Webhooks** - Trigger tasks from GitHub events
- **Dashboard** - Web UI showing all agents, tasks, PRs

## File Structure

```
examples/agent-swarm/
├── PROPOSAL.md           # This file
├── cmd/
│   ├── worker/main.go    # Worker CLI
│   └── swarm/main.go     # Orchestrator CLI
├── internal/
│   ├── agent/            # Agent discovery, metadata
│   ├── executor/         # Executor interface + implementations
│   │   ├── executor.go   # Interface
│   │   ├── claude.go     # Claude Code executor
│   │   └── codex.go      # OpenAI Codex executor (future)
│   ├── session/          # Session management
│   ├── worktree/         # Git worktree helpers
│   └── protocol/         # Message types
├── go.mod
└── README.md
```
