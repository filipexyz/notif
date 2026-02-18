# OPERATOR_SEED Rotation Runbook

The `OPERATOR_SEED` is the root of trust for the notif.sh NATS multi-account system. It signs all account JWTs. This document describes the procedure for rotating it.

## When to Rotate

- **Annually** as a preventive measure
- **Immediately** on suspected compromise
- **Before** decommissioning infrastructure that had access to the seed

## Prerequisites

- `nk` CLI (NATS key tool): `go install github.com/nats-io/nkeys/nk@latest`
- Admin access to the NATS server configuration
- Admin access to the notifd deployment configuration
- Current `OPERATOR_SEED` (to verify existing JWTs during transition)

## Procedure

### 1. Generate New Operator Key Pair

```bash
nk -gen operator
```

This outputs two values:
- **Seed** (starts with `SO`): the private key — this becomes the new `OPERATOR_SEED`
- **Public Key** (starts with `O`): used in NATS server config

Save both securely. The seed must never be committed to version control.

### 2. Update NATS Server Configuration

In `nats-server.conf`, update the `trusted_keys` array with the new operator public key:

```hcl
operator: {
    trusted_keys: [
        "OABC...NEW_PUBLIC_KEY..."
    ]
}
```

**Do NOT remove the old key yet** — existing JWTs are still signed with it.

### 3. Rebuild All Account JWTs

Use the CLI to rebuild every account JWT with the new operator key:

```bash
notif accounts rebuild-all --operator-seed "SOABC...NEW_SEED..."
```

This iterates every organization in the database and calls `RebuildAndPush` for each, signing new JWTs with the new operator key.

### 4. Verify All Account JWTs

```bash
notif accounts verify-all
```

This checks that every account JWT in NATS was signed by the current operator key. All accounts should report as valid.

### 5. Remove Old Operator Key

Once all JWTs are rebuilt and verified, remove the old operator public key from `trusted_keys` in `nats-server.conf`:

```hcl
operator: {
    trusted_keys: [
        "OABC...NEW_PUBLIC_KEY..."
    ]
}
```

Reload NATS server configuration:

```bash
nats-server --signal reload
```

### 6. Rotate Environment Variable

Update `OPERATOR_SEED` in the notifd deployment configuration:

- **Docker/Compose**: Update `.env` or `docker-compose.yaml`
- **Kubernetes**: Update the Secret resource
- **Systemd**: Update the environment file

### 7. Restart notifd

```bash
# Docker
docker restart notifd

# Kubernetes
kubectl rollout restart deployment/notifd

# Systemd
systemctl restart notifd
```

notifd will boot with the new `OPERATOR_SEED` and use it for all future JWT operations.

### 8. Post-Rotation Verification

```bash
# Verify all accounts are healthy
notif accounts verify-all

# Check audit log for the rotation event
notif audit --action operator.rotate --since 1h
```

## Audit Trail

The rotation is automatically logged as `operator.rotate` in the audit log with:
- **actor**: `cli:admin`
- **action**: `operator.rotate`
- **detail**: `{"accounts_rebuilt": N}`

## Rollback

If something goes wrong during rotation:

1. Re-add the old operator public key to `trusted_keys`
2. Set `OPERATOR_SEED` back to the old seed
3. Run `notif accounts rebuild-all` with the old seed
4. Restart notifd

## Security Notes

- Never store `OPERATOR_SEED` in version control
- Use a secrets manager (Vault, AWS Secrets Manager, etc.) in production
- The seed should only be accessible to the notifd process and authorized operators
- All rotation operations are logged in the audit trail
