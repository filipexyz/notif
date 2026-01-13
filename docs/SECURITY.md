# Security & Access Control

notif.sh implements filesystem-based topic-level access control policies for fine-grained authorization.

## Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    FILESYSTEM-BASED SECURITY                                │
└─────────────────────────────────────────────────────────────────────────────┘

  Client (untrusted)              │           Server (trusted)
                                  │
  notif emit filipe.tool.request  │    ┌─────────────────────────┐
  ─────────────────────────────────────▶│  1. Check identity      │
                                  │    │  2. Read policy from    │
         ✓ ALLOWED                │    │     /etc/notif/policies/│
  ◀─────────────────────────────────────│  3. Enforce rules       │
                                  │    └─────────────────────────┘
```

## Features

- **Filesystem-based policies**: Rules stored as YAML files, protected by OS permissions
- **Hot-reload**: Policies update automatically when files change (no restart required)
- **Multi-tenant**: Per-organization policy isolation
- **Wildcard patterns**: Flexible topic and identity matching
- **Audit logging**: Security events published to NATS for monitoring
- **Backward compatible**: No policies = allow by default

## Policy File Format

```yaml
org_id: "your-org-id"
version: "1.0"
description: "Policy description"
updated_at: 2024-01-13T00:00:00Z

# Deny by default (require explicit allow)
default_deny: true

# Audit settings
audit_enabled: true
audit_topic: "security.audit"
audit_denied_only: false  # Log both allowed and denied
audit_include_data: false  # Don't log event payloads (privacy)

topics:
  - pattern: "user.*"
    description: "User events"
    publish:
      - identity: "backend-*"
        type: "api_key"
        description: "Backend services"
    subscribe:
      - identity: "*"
        description: "Anyone can subscribe"
```

## Pattern Matching

### Topic Patterns

| Pattern | Matches | Doesn't Match |
|---------|---------|---------------|
| `user.created` | `user.created` | `user.updated` |
| `user.*` | `user.created`, `user.updated` | `user.profile.updated` |
| `user.>` | `user.created`, `user.profile.updated` | `user` (needs at least one segment after) |
| `*.created` | `user.created`, `order.created` | `user.profile.created` |

### Identity Patterns

| Pattern | Matches | Doesn't Match |
|---------|---------|---------------|
| `worker-123` | `worker-123` | `worker-456` |
| `worker-*` | `worker-1`, `worker-abc` | `worker` |
| `*-prod` | `api-prod`, `service-1-prod` | `prod` |
| `*` | Anything | - |

## Principal Types

Two types of principals:

1. **API Keys** (`type: api_key`): Service-to-service authentication
   - Identified by API key ID (UUID)
   - Created via `/api/v1/api-keys`
   - Format: `nsh_[a-zA-Z0-9]{28}`

2. **Users** (`type: user`): Human users via Clerk JWT
   - Identified by Clerk user ID
   - Authenticated via dashboard

## Configuration

### Environment Variables

```bash
# Policy directory (default: /etc/notif/policies)
NOTIF_POLICY_DIR=/custom/path/to/policies
```

### Policy Directory Structure

```
/etc/notif/policies/
├── org_abc123.yaml       # Policy for org_abc123
├── org_xyz789.yaml       # Policy for org_xyz789
└── filipe-ai.yaml        # Policy for filipe-ai
```

### File Permissions

Policies should be read-only for the notif server:

```bash
sudo chown root:notif /etc/notif/policies/*.yaml
sudo chmod 640 /etc/notif/policies/*.yaml
```

## Examples

### Example 1: Event-Driven Tool Pipeline (filipe aí)

```yaml
org_id: "filipe-ai"
version: "1.0"
default_deny: true
audit_enabled: true

topics:
  # Tool requests - bot → workers
  - pattern: "filipe.tool.request"
    publish:
      - identity: "filipe-bot"
        type: "api_key"
    subscribe:
      - identity: "claude-worker-*"
        type: "api_key"

  # Tool responses - workers → bot
  - pattern: "filipe.tool.response"
    publish:
      - identity: "claude-worker-*"
        type: "api_key"
    subscribe:
      - identity: "filipe-bot"
        type: "api_key"

  # Private user data - bot only
  - pattern: "filipe.user.>"
    publish:
      - identity: "filipe-bot"
        type: "api_key"
    subscribe:
      - identity: "filipe-bot"
        type: "api_key"
```

### Example 2: Multi-Service Architecture

```yaml
org_id: "acme-corp"
version: "1.0"
default_deny: false  # Allow by default

topics:
  # Admin topics - restricted
  - pattern: "admin.>"
    publish:
      - identity: "admin-*"
        type: "api_key"
    subscribe:
      - identity: "admin-*"
        type: "api_key"

  # Public topics - open to all
  - pattern: "public.>"
    publish:
      - identity: "*"
    subscribe:
      - identity: "*"
```

### Example 3: Development Environment

```yaml
org_id: "dev"
version: "1.0"
default_deny: false  # Allow everything
audit_enabled: true
audit_denied_only: false  # Log all access for debugging
audit_include_data: true  # Include payloads for debugging

topics:
  # Only restrict destructive operations
  - pattern: "admin.delete.*"
    publish:
      - identity: "admin-*"
        type: "api_key"
```

## Audit Logging

When enabled, security events are published to a special audit topic.

### Audit Event Format

```json
{
  "timestamp": "2024-01-13T12:34:56Z",
  "org_id": "acme-corp",
  "principal": {
    "type": "api_key",
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "org_id": "acme-corp"
  },
  "action": "publish",
  "topic": "user.created",
  "result": "denied",
  "reason": "no matching rule for topic \"user.created\"",
  "matched_rule": null
}
```

### Subscribing to Audit Events

```bash
# Watch all security audit events
notif subscribe security.audit

# Filter for denied attempts only
notif subscribe security.audit --filter '.result == "denied"'

# Count denied attempts per topic
notif subscribe security.audit --json | jq -s 'group_by(.topic) | map({topic: .[0].topic, denials: length})'
```

## Migration Guide

### Enabling Policies for Existing Deployments

1. **Create policy directory**:
   ```bash
   sudo mkdir -p /etc/notif/policies
   sudo chown notif:notif /etc/notif/policies
   ```

2. **Start with permissive policy** (don't break existing systems):
   ```yaml
   org_id: "your-org-id"
   version: "1.0"
   default_deny: false  # Allow by default
   audit_enabled: true
   audit_denied_only: false

   topics: []  # No restrictions yet
   ```

3. **Monitor audit logs** to understand access patterns:
   ```bash
   notif subscribe security.audit --json > audit.log
   ```

4. **Gradually add restrictions**:
   - Start with `default_deny: false` + specific deny rules
   - Monitor for denials
   - Switch to `default_deny: true` + allow rules when ready

### Testing Policies

Before deploying:

1. **Validate YAML syntax**:
   ```bash
   yamllint /etc/notif/policies/your-org.yaml
   ```

2. **Check server logs** after saving policy file:
   ```bash
   journalctl -u notif -f
   # Look for: "INFO: Loaded policy for org your-org from your-org.yaml"
   ```

3. **Test access** with different principals:
   ```bash
   # Try to publish with API key
   notif emit --api-key nsh_xxx test.topic '{"data": "test"}'

   # Check audit log
   notif subscribe security.audit
   ```

## Security Best Practices

1. **Use `default_deny: true`** for sensitive organizations
2. **Principle of least privilege**: Grant minimum required access
3. **Wildcard patterns carefully**: `*` matches everything
4. **Enable audit logging**: Track all access (or at least denials)
5. **Rotate policies regularly**: Review and update access rules
6. **Monitor audit logs**: Alert on suspicious patterns
7. **Protect policy files**: Use OS permissions to prevent tampering

## Troubleshooting

### Policy Not Loading

Check server logs:
```bash
journalctl -u notif -n 100 | grep policy
```

Common issues:
- Invalid YAML syntax
- Missing `org_id` field
- File permissions prevent reading
- Wrong policy directory (check `NOTIF_POLICY_DIR`)

### Access Denied Unexpectedly

1. Check audit logs:
   ```bash
   notif subscribe security.audit
   ```

2. Verify policy file:
   ```bash
   cat /etc/notif/policies/your-org.yaml
   ```

3. Test topic matching:
   - Ensure pattern covers your topic
   - Check for `default_deny: true` without matching rule

4. Verify principal identity:
   - API key: Check API key ID in logs
   - User: Check Clerk user ID

### Hot Reload Not Working

Check if fsnotify is working:
```bash
# Trigger a reload
touch /etc/notif/policies/your-org.yaml

# Check logs
journalctl -u notif -f
# Should see: "INFO: Policy files changed, reloading..."
```

## Performance

- **Policy loading**: O(1) per request (in-memory cache)
- **Topic matching**: O(n) where n = number of topic policies (typically < 100)
- **Identity matching**: O(1) (simple string operations)
- **Audit logging**: Asynchronous (non-blocking)

For high-traffic scenarios:
- Keep policy files small (< 100 topic patterns)
- Use specific patterns over broad wildcards when possible
- Consider disabling audit logging or using `audit_denied_only: true`

## Further Reading

- [Policy File Examples](../examples/policies/)
- [Authentication Guide](./AUTH.md)
- [Multi-Tenancy](./ARCHITECTURE.md#multi-tenancy)
