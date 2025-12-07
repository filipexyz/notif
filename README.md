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

# Add notifications with tags
notif add -t work,urgent "Fix the production bug"
notif add -t personal "Call mom"

# List pending notifications
notif ls

# List filtered by tag
notif ls -t work

# Clear delivered notifications
notif clear
```

## Tags & Project Filtering

Tags allow you to categorize notifications and filter them per project.

### Adding Tags

```bash
notif add -t work "Work notification"
notif add -t work,urgent "Urgent work notification"
```

### Project Configuration

Create a `.notif.json` in your project root to filter which notifications appear:

```bash
notif init -t work,myproject
```

This creates:

```json
{
  "tags": ["work", "myproject"],
  "mode": "include"
}
```

- **include mode**: Only show notifications WITH at least one matching tag
- **exclude mode**: Hide notifications WITH any matching tag
- **Tag-less notifications** always show (they're global)

The hook reads `.notif.json` from `$CLAUDE_PROJECT_DIR` (set by Claude Code) or the current directory.

## How It Works

1. `notif add` stores notifications in a local SQLite database
2. A `UserPromptSubmit` hook fires before every message you send to Claude
3. The hook reads up to 3 pending notifications (high priority first)
4. If `.notif.json` exists, notifications are filtered by tags
5. Notifications are injected as context that Claude sees
6. Delivered notifications are marked so they don't repeat

## Hook Output

When you have pending notifications, Claude sees:

```
Pending notifications:
- [work, urgent] Fix the production bug
- [work] Review PR #42
- Remember to update README
```

## Commands

| Command | Description |
|---------|-------------|
| `notif add <message>` | Add a notification |
| `notif add -p high <message>` | Add high priority notification |
| `notif add -t work,urgent <message>` | Add notification with tags |
| `notif ls` | List pending notifications |
| `notif ls -t work` | List notifications filtered by tag |
| `notif hook` | Hook mode (called by Claude Code) |
| `notif clear` | Remove delivered notifications |
| `notif init` | Setup hook in `~/.claude/settings.json` |
| `notif init -t work` | Setup hook + create `.notif.json` with tags |

## Use Cases

- **CI/CD**: `notif add -p high -t ci "Build failed on main!"` from your pipeline
- **Per-project context**: Different projects see different notifications via `.notif.json`
- **Cron jobs**: Daily reminders or scheduled notes
- **Long-running tasks**: Notify yourself when a process completes
- **Team workflows**: Tag notifications by project or team

## Configuration

### Global Hook

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

### Project Filter

Optional `.notif.json` in project root:

```json
{
  "tags": ["work", "myproject"],
  "mode": "include"
}
```

## License

MIT
