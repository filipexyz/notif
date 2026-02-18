package accounts

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/filipexyz/notif/internal/audit"
	"github.com/filipexyz/notif/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nats-io/nkeys"
)

// Manager handles org/account lifecycle operations.
type Manager struct {
	queries    *db.Queries
	jwt        *JWTManager
	operatorKP nkeys.KeyPair
	auditLog   *audit.Logger
}

// NewManager creates a new account Manager.
func NewManager(queries *db.Queries, operatorKP nkeys.KeyPair, auditLog *audit.Logger) *Manager {
	jwtMgr := NewJWTManager(queries, operatorKP, auditLog)
	return &Manager{
		queries:    queries,
		jwt:        jwtMgr,
		operatorKP: operatorKP,
		auditLog:   auditLog,
	}
}

// JWTManager returns the underlying JWT manager.
func (m *Manager) JWTManager() *JWTManager {
	return m.jwt
}

// CreateOrgResult contains the result of creating an org.
type CreateOrgResult struct {
	Org        db.Org
	AccountKP  nkeys.KeyPair
	SignedJWT  string
}

// CreateOrg creates a new org with a NATS account.
// Returns the org, account key pair, and signed JWT.
// The caller is responsible for pushing the JWT and creating the pool connection.
func (m *Manager) CreateOrg(ctx context.Context, id, name string) (*CreateOrgResult, error) {
	// Generate NATS account key pair
	accountKP, err := GenerateAccountKey()
	if err != nil {
		return nil, fmt.Errorf("generate account key: %w", err)
	}

	pubKey, err := PublicKey(accountKP)
	if err != nil {
		return nil, fmt.Errorf("get public key: %w", err)
	}

	seedStr, err := Seed(accountKP)
	if err != nil {
		return nil, fmt.Errorf("get account seed: %w", err)
	}

	// Insert into DB
	org, err := m.queries.CreateOrg(ctx, db.CreateOrgParams{
		ID:              id,
		Name:            name,
		NatsPublicKey:   pubKey,
		NatsAccountSeed: pgtype.Text{String: seedStr, Valid: true},
		BillingTier:     pgtype.Text{String: "free", Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("create org in DB: %w", err)
	}

	// Build account JWT
	signed, err := m.jwt.BuildAccountJWT(ctx, id)
	if err != nil {
		// Rollback DB insert
		_ = m.queries.DeleteOrg(ctx, id)
		return nil, fmt.Errorf("build account JWT: %w", err)
	}

	// Audit log
	if m.auditLog != nil {
		m.auditLog.Log(ctx, "notifd", "account.create", id, id, map[string]any{
			"name":       name,
			"public_key": pubKey,
		})
	}

	slog.Info("org created", "org_id", id, "name", name, "nats_pub_key", pubKey)

	return &CreateOrgResult{
		Org:       org,
		AccountKP: accountKP,
		SignedJWT: signed,
	}, nil
}

// DeleteOrg deletes an org and revokes its NATS account.
// The caller is responsible for removing the pool connection.
func (m *Manager) DeleteOrg(ctx context.Context, orgID string) error {
	// Get org to verify it exists
	org, err := m.queries.GetOrg(ctx, orgID)
	if err != nil {
		return fmt.Errorf("get org %s: %w", orgID, err)
	}

	// Delete from DB (cascades to projects, api_keys via FK)
	if err := m.queries.DeleteOrg(ctx, orgID); err != nil {
		return fmt.Errorf("delete org from DB: %w", err)
	}

	// Audit log
	if m.auditLog != nil {
		m.auditLog.Log(ctx, "notifd", "account.delete", orgID, orgID, map[string]any{
			"name":       org.Name,
			"public_key": org.NatsPublicKey,
		})
	}

	slog.Info("org deleted", "org_id", orgID)
	return nil
}

// UpdateBillingTier updates an org's billing tier and returns the updated org.
// The caller is responsible for rebuilding and pushing the JWT.
func (m *Manager) UpdateBillingTier(ctx context.Context, orgID, tier string) (db.Org, error) {
	org, err := m.queries.UpdateOrgBillingTier(ctx, db.UpdateOrgBillingTierParams{
		ID:          orgID,
		BillingTier: pgtype.Text{String: tier, Valid: true},
	})
	if err != nil {
		return db.Org{}, fmt.Errorf("update billing tier: %w", err)
	}

	// Audit log
	if m.auditLog != nil {
		m.auditLog.Log(ctx, "notifd", "tier.change", orgID, orgID, map[string]any{
			"new_tier": tier,
		})
	}

	return org, nil
}
