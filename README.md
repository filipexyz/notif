# notif

A notification center for Claude Code sessions. Push notifications from anywhere and have them injected into your next Claude interaction.

## Installation

```bash
cargo install --path .
notif init  # configures the Claude Code hook
```

## Usage

```bash
# Add notifications
notif add "Remember to review PR #42"
notif add -p high "URGENT: Deploy to prod!"
notif add -p low "Update docs someday"

# List pending notifications
notif ls

# Clear delivered notifications
notif clear
```

## How It Works

1. `notif add` stores notifications in a local SQLite database (`~/.local/share/com.filipelabs.notif/notif.db`)
2. A `UserPromptSubmit` hook fires before every message you send to Claude
3. The hook reads up to 3 pending notifications (high priority first)
4. Notifications are injected as context that Claude sees
5. Delivered notifications are marked so they don't repeat

## Hook Output

When you have pending notifications, Claude sees:

```
Pending notifications:
- URGENT: Deploy to prod!
- Remember to review PR #42
- Update docs someday
```

## Commands

| Command | Description |
|---------|-------------|
| `notif add <message>` | Add a notification |
| `notif add -p high <message>` | Add high priority notification |
| `notif add -p low <message>` | Add low priority notification |
| `notif ls` | List pending notifications |
| `notif hook` | Hook mode (called by Claude Code) |
| `notif clear` | Remove delivered notifications |
| `notif init` | Setup hook in `~/.claude/settings.json` |

## Use Cases

- **CI/CD**: `notif add -p high "Build failed on main!"` from your pipeline
- **Cron jobs**: Daily reminders or scheduled notes
- **Long-running tasks**: Notify yourself when a process completes
- **Manual notes**: Jot down things to remember mid-session

## Configuration

The hook is configured in `~/.claude/settings.json`:

```json
{
  "hooks": {
    "UserPromptSubmit": [{
      "hooks": [{
        "type": "command",
        "command": "notif hook"
      }]
    }]
  }
}
```

## License

MIT
