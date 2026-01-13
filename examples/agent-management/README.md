# Agent Management

A desktop app for managing Claude Code agents and permissions.

## Features

- **Permission Queue**: Handles permission requests from multiple Claude Code sessions
- **Per-Session Queues**: Each session has its own queue of pending permissions
- **Diff View**: Syntax-highlighted diff view for Edit operations
- **Keyboard Shortcuts**: Enter (Allow), Esc (Deny)
- **Auto-Timeout**: Auto-denies after 45 seconds of inactivity

## Setup

1. Set your notif API key:
```bash
export NOTIF_API_KEY=nsh_xxx
```

2. Run the app:
```bash
cargo tauri dev
```

3. Configure Claude Code hook in `~/.claude/settings.json`:
```json
{
  "hooks": {
    "PermissionRequest": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "notif emit claude.permission.request --reply-to claude.permission.response --filter '.session_id == $input.session_id' --timeout 60s --json --raw",
            "timeout": 65
          }
        ]
      }
    ]
  }
}
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   Agent Management                       │
│  ┌──────────────┐  ┌──────────────────────────────────┐ │
│  │   Sessions   │  │      Current Permission          │ │
│  │              │  │                                  │ │
│  │ • session-1  │  │  Edit: src/lib.rs               │ │
│  │   (2 pending)│  │  ┌────────────────────────────┐ │ │
│  │              │  │  │ - old line                 │ │ │
│  │ • session-2  │  │  │ + new line                 │ │ │
│  │   (1 pending)│  │  └────────────────────────────┘ │ │
│  │              │  │                                  │ │
│  └──────────────┘  │  [Deny]            [Allow]      │ │
│                    └──────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
         ▲                                    │
         │ subscribe                          │ emit
         │ claude.permission.request          │ claude.permission.response
         │                                    ▼
    ┌────────────────────────────────────────────────┐
    │                  notif.sh                       │
    └────────────────────────────────────────────────┘
         ▲                                    │
         │ emit                               │ filter by session_id
         │                                    ▼
    ┌────────────────────────────────────────────────┐
    │              Claude Code (hooks)                │
    │  Session 1        Session 2        Session 3   │
    └────────────────────────────────────────────────┘
```

## Tech Stack

- **Tauri 2.0**: Desktop framework
- **Rust**: Backend
- **notifsh**: Rust SDK for notif.sh
- **diff.js**: Line/word diff calculation
- **highlight.js**: Syntax highlighting
