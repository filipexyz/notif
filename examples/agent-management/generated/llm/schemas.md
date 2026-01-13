# Available Schemas

This document describes all available schemas in the system.

## AgentEvent

Event emitted by an agent (progress, completion, errors)

**Version:** 1.0.0

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| sessionId | string | yes | Session identifier |
| agent | string | yes | Agent name that emitted this event |
| kind | enum (started, progress, output, completed, failed, blocked) | yes | Event type |
| message | string | no | Human-readable progress message |
| result | string | no | Final result summary (for completed) |
| error | object | no | Error details (for failed) |
| error.message | string | no |  |
| error.code | string | no |  |
| pr | object | no | PR info if created (for completed) |
| pr.url | string | no |  |
| pr.number | integer | no |  |
| costUsd | number | no | Cost incurred so far |
| timestamp | string (ISO datetime) | yes |  |

### Example

```json
{
  "sessionId": "example-sessionId",
  "agent": "example-agent",
  "kind": "started",
  "message": "example-message",
  "result": "example-result",
  "error": {
    "message": "example-message",
    "code": "example-code"
  },
  "pr": {
    "url": "example-url",
    "number": 42
  },
  "costUsd": 3.14,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Agent

An agent that can be discovered and controlled remotely

**Version:** 1.0.0

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | yes | Unique agent name (e.g., "coder-1", "researcher") |
| description | string | no | What this agent does |
| hostname | string | no | Machine hostname where agent is running |
| tags | string[] | no | Capabilities/skills tags (e.g., ["typescript", "react"]) |
| executor | object | no |  |
| executor.kind | enum (claude, codex, gemini, custom) | no | The executor type |
| executor.version | string | no | Model or CLI version |
| executor.cli | string | no | Command to invoke (e.g., "claude") |
| project | object | no |  |
| project.name | string | no |  |
| project.path | string | no | Working directory |
| project.repo | string | no | Git repo (e.g., "filipexyz/notif") |
| status | enum (idle, busy, offline) | no | Current agent status (default: "idle") |
| activeSessionId | string | no | Currently running session ID if busy |

### Example

```json
{
  "name": "example-name",
  "description": "example-description",
  "hostname": "example-hostname",
  "tags": [
    "example-item"
  ],
  "executor": {
    "kind": "claude",
    "version": "example-version",
    "cli": "example-cli"
  },
  "project": {
    "name": "example-name",
    "path": "example-path",
    "repo": "example-repo"
  },
  "status": "idle",
  "activeSessionId": "example-activeSessionId"
}
```

## PermissionRequest

Permission request from Claude Code hook

**Version:** 1.0.0

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | no | Unique identifier for this request (from event ID) |
| toolName | string | no | Name of the tool requesting permission (Edit, Write, Bash, Read, etc.) |
| toolInput | object | no | Tool-specific input parameters |
| sessionId | string | no | Claude Code session identifier |
| cwd | string | no | Current working directory of the Claude Code session |

### Example

```json
{
  "id": "example-id",
  "toolName": "example-toolName",
  "toolInput": {},
  "sessionId": "example-sessionId",
  "cwd": "example-cwd"
}
```

## SessionInfo

Session information for the UI

**Version:** 1.0.0

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| sessionId | string | yes | Claude Code session identifier |
| cwd | string | no | Current working directory of the session |
| queueCount | integer | yes | Number of pending permission requests in this session's queue (min: 0) |

### Example

```json
{
  "sessionId": "example-sessionId",
  "cwd": "example-cwd",
  "queueCount": 42
}
```

## PermissionResponse

Permission response sent back to Claude Code hook

**Version:** 1.0.0

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| sessionId | string | no | Claude Code session identifier |
| hookSpecificOutput | object | yes |  |
| hookSpecificOutput.hookEventName | string | yes | Always "PermissionRequest" |
| hookSpecificOutput.decision | object | yes |  |
| hookSpecificOutput.decision.behavior | enum (allow, deny) | yes | Whether to allow or deny the permission |
| hookSpecificOutput.decision.message | string | no | Denial message (only used when behavior is deny) |

### Example

```json
{
  "sessionId": "example-sessionId",
  "hookSpecificOutput": {
    "hookEventName": "example-hookEventName",
    "decision": {
      "behavior": "allow",
      "message": "example-message"
    }
  }
}
```

## AgentMessage

Message sent to control an agent (prompt, cancel, etc.)

**Version:** 1.0.0

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| sessionId | string | yes | Session identifier for tracking |
| agent | string | yes | Target agent name |
| kind | enum (prompt, resume, cancel) | yes | Message type |
| prompt | string | no | The prompt/instruction to send (for prompt/resume) |
| options | object | no |  |
| options.budgetUsd | number | no | Max spend for this session |
| options.timeoutSeconds | integer | no | Session timeout |
| options.autoPr | boolean | no | Auto-create PR on completion |

### Example

```json
{
  "sessionId": "example-sessionId",
  "agent": "example-agent",
  "kind": "prompt",
  "prompt": "example-prompt",
  "options": {
    "budgetUsd": 3.14,
    "timeoutSeconds": 42,
    "autoPr": true
  }
}
```
