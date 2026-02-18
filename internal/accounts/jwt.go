package accounts

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/filipexyz/notif/internal/audit"
	"github.com/filipexyz/notif/internal/db"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

// JWTManager handles JWT generation and pushing for NATS accounts.
type JWTManager struct {
	queries    *db.Queries
	operatorKP nkeys.KeyPair
	auditLog   *audit.Logger
}

// NewJWTManager creates a new JWTManager.
func NewJWTManager(queries *db.Queries, operatorKP nkeys.KeyPair, auditLog *audit.Logger) *JWTManager {
	return &JWTManager{
		queries:    queries,
		operatorKP: operatorKP,
		auditLog:   auditLog,
	}
}

// BuildAccountJWT builds an account JWT from DB state (JWT-as-derived-view).
// Never stores JWTs — always rebuilds from source of truth.
func (m *JWTManager) BuildAccountJWT(ctx context.Context, orgID string) (string, error) {
	org, err := m.queries.GetOrg(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("get org %s: %w", orgID, err)
	}

	claims := jwt.NewAccountClaims(org.NatsPublicKey)
	claims.Name = org.Name

	// Apply limits from billing tier
	tier := "free"
	if org.BillingTier.Valid {
		tier = org.BillingTier.String
	}
	limits := DefaultTierLimits(tier)
	ApplyLimits(claims, limits)

	// Enable JetStream for the account
	claims.Limits.JetStreamLimits = jwt.JetStreamLimits{
		MemoryStorage: -1, // use server defaults
		DiskStorage:   limits.StreamMaxBytes,
		Streams:       -1,
		Consumer:      -1,
	}

	signed, err := claims.Encode(m.operatorKP)
	if err != nil {
		return "", fmt.Errorf("encode account JWT for %s: %w", orgID, err)
	}

	return signed, nil
}

// BuildSystemAccountJWT builds the system account JWT.
func (m *JWTManager) BuildSystemAccountJWT(systemPubKey string) (string, error) {
	claims := jwt.NewAccountClaims(systemPubKey)
	claims.Name = "SYS"
	// System account gets unlimited access
	claims.Limits.Conn = -1
	claims.Limits.Data = -1
	claims.Limits.Payload = -1
	claims.Limits.Exports = -1
	claims.Limits.Imports = -1
	claims.Limits.Subs = -1

	signed, err := claims.Encode(m.operatorKP)
	if err != nil {
		return "", fmt.Errorf("encode system account JWT: %w", err)
	}
	return signed, nil
}

// BuildUserJWT builds a user JWT for connecting to a specific account.
func (m *JWTManager) BuildUserJWT(userPubKey string, accountKP nkeys.KeyPair) (string, error) {
	accountPub, err := accountKP.PublicKey()
	if err != nil {
		return "", fmt.Errorf("get account public key: %w", err)
	}

	claims := jwt.NewUserClaims(userPubKey)
	claims.IssuerAccount = accountPub

	signed, err := claims.Encode(accountKP)
	if err != nil {
		return "", fmt.Errorf("encode user JWT: %w", err)
	}
	return signed, nil
}

// RebuildAndPushAccountJWT rebuilds an account JWT from DB state and pushes it.
func (m *JWTManager) RebuildAndPushAccountJWT(ctx context.Context, orgID string, sysConn *nats.Conn) error {
	signed, err := m.BuildAccountJWT(ctx, orgID)
	if err != nil {
		return err
	}

	if err := pushJWT(sysConn, signed); err != nil {
		return fmt.Errorf("push JWT for %s: %w", orgID, err)
	}

	// Audit log
	if m.auditLog != nil {
		m.auditLog.Log(ctx, "notifd", "jwt.push", orgID, orgID, map[string]any{
			"operation": "rebuild_and_push",
		})
	}

	slog.Info("account JWT pushed", "org_id", orgID)
	return nil
}

// RebuildAndPushMultipleAccounts atomically pushes JWTs for multiple accounts.
// If any push fails, previously pushed accounts are rolled back.
func (m *JWTManager) RebuildAndPushMultipleAccounts(ctx context.Context, orgIDs []string, sysConn *nats.Conn) error {
	// Phase 1: Build all JWTs (no side effects)
	jwts := make(map[string]string)
	for _, orgID := range orgIDs {
		signed, err := m.BuildAccountJWT(ctx, orgID)
		if err != nil {
			return fmt.Errorf("build JWT for %s: %w", orgID, err)
		}
		jwts[orgID] = signed
	}

	// Phase 2: Push all JWTs
	var pushed []string
	for orgID, signed := range jwts {
		if err := pushJWT(sysConn, signed); err != nil {
			// Rollback: rebuild and push already-pushed accounts without the new change
			slog.Error("JWT push failed, rolling back", "org", orgID, "pushed", pushed)
			m.rollbackJWTPush(ctx, pushed, sysConn)
			return fmt.Errorf("push JWT for %s: %w (rolled back %v)", orgID, err, pushed)
		}
		pushed = append(pushed, orgID)

		// Audit log each push
		if m.auditLog != nil {
			m.auditLog.Log(ctx, "notifd", "jwt.push", orgID, orgID, map[string]any{
				"operation":   "transactional_push",
				"batch_size":  len(orgIDs),
				"batch_index": len(pushed),
			})
		}
	}

	return nil
}

// rollbackJWTPush rebuilds and pushes JWTs for already-pushed orgs to restore prior state.
func (m *JWTManager) rollbackJWTPush(ctx context.Context, pushedOrgIDs []string, sysConn *nats.Conn) {
	for _, orgID := range pushedOrgIDs {
		signed, err := m.BuildAccountJWT(ctx, orgID)
		if err != nil {
			slog.Error("rollback: failed to build JWT", "org", orgID, "error", err)
			continue
		}
		if err := pushJWT(sysConn, signed); err != nil {
			slog.Error("rollback: failed to push JWT", "org", orgID, "error", err)
		}
	}
}

// pushJWT publishes a signed JWT to $SYS.REQ.CLAIMS.UPDATE.
func pushJWT(sysConn *nats.Conn, signedJWT string) error {
	resp, err := sysConn.Request("$SYS.REQ.CLAIMS.UPDATE", []byte(signedJWT), 5*time.Second)
	if err != nil {
		return fmt.Errorf("request claims update: %w", err)
	}

	// Check response for errors
	respData := string(resp.Data)
	if len(respData) > 0 && respData[0] == '-' {
		return fmt.Errorf("claims update rejected: %s", respData)
	}

	return nil
}
