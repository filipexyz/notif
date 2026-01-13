# Schema Definitions

You are working with the following schemas. Always validate your outputs against these definitions.

## AgentEvent (v1.0.0)

Event emitted by an agent (progress, completion, errors)

**Required fields:**
- `sessionId` (string) - Session identifier
- `agent` (string) - Agent name that emitted this event
- `kind` (enum (started, progress, output, completed, failed, blocked)) - Event type
- `timestamp` (string (ISO datetime))

**Optional fields:**
- `message` (string) - Human-readable progress message
- `result` (string) - Final result summary (for completed)
- `error` (object) - Error details (for failed)
- `error.message` (string)
- `error.code` (string)
- `pr` (object) - PR info if created (for completed)
- `pr.url` (string)
- `pr.number` (integer)
- `costUsd` (number) - Cost incurred so far

**Example:**
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

## Agent (v1.0.0)

An agent that can be discovered and controlled remotely

**Required fields:**
- `name` (string) - Unique agent name (e.g., "coder-1", "researcher")

**Optional fields:**
- `description` (string) - What this agent does
- `hostname` (string) - Machine hostname where agent is running
- `tags` (string[]) - Capabilities/skills tags (e.g., ["typescript", "react"])
- `executor` (object)
- `executor.kind` (enum (claude, codex, gemini, custom)) - The executor type
- `executor.version` (string) - Model or CLI version
- `executor.cli` (string) - Command to invoke (e.g., "claude")
- `project` (object)
- `project.name` (string)
- `project.path` (string) - Working directory
- `project.repo` (string) - Git repo (e.g., "filipexyz/notif")
- `status` (enum (idle, busy, offline)) [default: "idle"] - Current agent status
- `activeSessionId` (string) - Currently running session ID if busy

**Example:**
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

## PermissionRequest (v1.0.0)

Permission request from Claude Code hook

**Required fields:**
- None

**Optional fields:**
- `id` (string) - Unique identifier for this request (from event ID)
- `toolName` (string) - Name of the tool requesting permission (Edit, Write, Bash, Read, etc.)
- `toolInput` (object) - Tool-specific input parameters
- `sessionId` (string) - Claude Code session identifier
- `cwd` (string) - Current working directory of the Claude Code session

**Example:**
```json
{
  "id": "example-id",
  "toolName": "example-toolName",
  "toolInput": {},
  "sessionId": "example-sessionId",
  "cwd": "example-cwd"
}
```

## SessionInfo (v1.0.0)

Session information for the UI

**Required fields:**
- `sessionId` (string) - Claude Code session identifier
- `queueCount` (integer) - Number of pending permission requests in this session's queue

**Optional fields:**
- `cwd` (string) - Current working directory of the session

**Constraints:**
- `queueCount`: min: 0

**Example:**
```json
{
  "sessionId": "example-sessionId",
  "cwd": "example-cwd",
  "queueCount": 42
}
```

## PermissionResponse (v1.0.0)

Permission response sent back to Claude Code hook

**Required fields:**
- `hookSpecificOutput` (object)
- `hookSpecificOutput.hookEventName` (string) - Always "PermissionRequest"
- `hookSpecificOutput.decision` (object)
- `hookSpecificOutput.decision.behavior` (enum (allow, deny)) - Whether to allow or deny the permission

**Optional fields:**
- `sessionId` (string) - Claude Code session identifier
- `hookSpecificOutput.decision.message` (string) - Denial message (only used when behavior is deny)

**Example:**
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

## AgentMessage (v1.0.0)

Message sent to control an agent (prompt, cancel, etc.)

**Required fields:**
- `sessionId` (string) - Session identifier for tracking
- `agent` (string) - Target agent name
- `kind` (enum (prompt, resume, cancel)) - Message type

**Optional fields:**
- `prompt` (string) - The prompt/instruction to send (for prompt/resume)
- `options` (object)
- `options.budgetUsd` (number) - Max spend for this session
- `options.timeoutSeconds` (integer) - Session timeout
- `options.autoPr` (boolean) - Auto-create PR on completion

**Example:**
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
