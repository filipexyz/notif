package nats

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/filipexyz/notif/internal/accounts"
	"github.com/filipexyz/notif/internal/audit"
	"github.com/filipexyz/notif/internal/db"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
	"golang.org/x/sync/errgroup"
)

// OrgClient wraps a per-account NATS connection and JetStream.
type OrgClient struct {
	orgID  string
	conn   *nats.Conn
	js     jetstream.JetStream
	stream jetstream.Stream // NOTIF_EVENTS_{orgID}
}

// JetStream returns the JetStream context for this org.
func (c *OrgClient) JetStream() jetstream.JetStream {
	return c.js
}

// Stream returns the main events stream for this org.
func (c *OrgClient) Stream() jetstream.Stream {
	return c.stream
}

// IsConnected returns true if the connection is active.
func (c *OrgClient) IsConnected() bool {
	return c.conn.IsConnected()
}

// Close drains and closes the connection.
func (c *OrgClient) Close() {
	c.conn.Drain()
}

// OrgID returns the org ID this client belongs to.
func (c *OrgClient) OrgID() string {
	return c.orgID
}

// ClientPool manages system + per-account NATS connections.
type ClientPool struct {
	natsURL    string
	system     *nats.Conn           // $SYS only — monitoring, claims management
	clients    map[string]*OrgClient // per-account connections
	mu         sync.RWMutex
	operatorKP nkeys.KeyPair
	jwtMgr     *accounts.JWTManager
	auditLog   *audit.Logger

	// Track account key pairs for user JWT generation (in-memory only)
	accountKeys map[string]nkeys.KeyPair // orgID -> account key pair
}

// NewClientPool creates a new ClientPool.
func NewClientPool(natsURL string, operatorKP nkeys.KeyPair, jwtMgr *accounts.JWTManager, auditLog *audit.Logger) *ClientPool {
	return &ClientPool{
		natsURL:     natsURL,
		clients:     make(map[string]*OrgClient),
		operatorKP:  operatorKP,
		jwtMgr:      jwtMgr,
		auditLog:    auditLog,
		accountKeys: make(map[string]nkeys.KeyPair),
	}
}

// SystemConn returns the system NATS connection.
func (p *ClientPool) SystemConn() *nats.Conn {
	return p.system
}

// ConnectSystem establishes the system account connection.
// This must succeed before any other operations.
func (p *ClientPool) ConnectSystem(ctx context.Context, systemAccountKP nkeys.KeyPair) error {
	systemPub, err := systemAccountKP.PublicKey()
	if err != nil {
		return fmt.Errorf("get system account public key: %w", err)
	}

	// Generate user key for system connection
	userKP, err := accounts.GenerateUserKey()
	if err != nil {
		return fmt.Errorf("generate system user key: %w", err)
	}

	userPub, err := userKP.PublicKey()
	if err != nil {
		return fmt.Errorf("get system user public key: %w", err)
	}

	// Build system user JWT
	userClaims := jwt.NewUserClaims(userPub)
	userClaims.IssuerAccount = systemPub
	userClaims.Name = "notifd-system"

	userJWT, err := userClaims.Encode(systemAccountKP)
	if err != nil {
		return fmt.Errorf("encode system user JWT: %w", err)
	}

	// Build system account JWT and push it
	sysAccountJWT, err := p.jwtMgr.BuildSystemAccountJWT(systemPub)
	if err != nil {
		return fmt.Errorf("build system account JWT: %w", err)
	}

	// Get seed string for NKey auth
	userSeed, err := userKP.Seed()
	if err != nil {
		return fmt.Errorf("get system user seed: %w", err)
	}

	// Connect with NKey auth
	nc, err := nats.Connect(p.natsURL,
		nats.UserJWTAndSeed(userJWT, string(userSeed)),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(5),
		nats.ReconnectWait(time.Second),
		nats.Name("notifd-system"),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Warn("system NATS disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			slog.Info("system NATS reconnected")
		}),
	)
	if err != nil {
		return fmt.Errorf("connect system account: %w", err)
	}

	p.system = nc

	// Push system account JWT
	resp, err := nc.Request("$SYS.REQ.CLAIMS.UPDATE", []byte(sysAccountJWT), 5*time.Second)
	if err != nil {
		slog.Warn("could not push system account JWT (may already exist)", "error", err)
	} else {
		slog.Debug("system account JWT pushed", "response", string(resp.Data))
	}

	slog.Info("system NATS connected", "url", p.natsURL)

	// Audit log
	if p.auditLog != nil {
		p.auditLog.Log(ctx, "notifd", "credential.rotate", "", "system", map[string]any{
			"user_pub_key": userPub,
		})
	}

	return nil
}

// Add creates a per-account connection for an org.
// It generates ephemeral NKey credentials, creates a user JWT, connects, and ensures streams.
func (p *ClientPool) Add(ctx context.Context, orgID string, accountKP nkeys.KeyPair) error {
	// Check for duplicate under read lock (fast path)
	p.mu.RLock()
	_, exists := p.clients[orgID]
	p.mu.RUnlock()
	if exists {
		return fmt.Errorf("org %s already in pool", orgID)
	}

	// All I/O happens outside the write lock
	accountPub, err := accountKP.PublicKey()
	if err != nil {
		return fmt.Errorf("get account public key for %s: %w", orgID, err)
	}

	// Generate ephemeral user key (in-memory only, regenerated on restart)
	userKP, err := accounts.GenerateUserKey()
	if err != nil {
		return fmt.Errorf("generate user key for %s: %w", orgID, err)
	}

	userPub, err := userKP.PublicKey()
	if err != nil {
		return fmt.Errorf("get user public key for %s: %w", orgID, err)
	}

	// Build user JWT
	userClaims := jwt.NewUserClaims(userPub)
	userClaims.IssuerAccount = accountPub
	userClaims.Name = "notifd-" + orgID

	userJWT, err := userClaims.Encode(accountKP)
	if err != nil {
		return fmt.Errorf("encode user JWT for %s: %w", orgID, err)
	}

	// Get seed string for NKey auth
	userSeed, err := userKP.Seed()
	if err != nil {
		return fmt.Errorf("get user seed for %s: %w", orgID, err)
	}

	// Connect (network I/O — outside lock)
	nc, err := nats.Connect(p.natsURL,
		nats.UserJWTAndSeed(userJWT, string(userSeed)),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.Name("notifd-"+orgID),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Warn("NATS disconnected", "org_id", orgID, "error", err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			slog.Info("NATS reconnected", "org_id", orgID)
		}),
	)
	if err != nil {
		return fmt.Errorf("connect org %s: %w", orgID, err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return fmt.Errorf("create JetStream for %s: %w", orgID, err)
	}

	// Ensure per-account streams (network I/O — outside lock)
	stream, err := ensureStreamsForOrg(ctx, js, orgID)
	if err != nil {
		nc.Close()
		return fmt.Errorf("ensure streams for %s: %w", orgID, err)
	}

	client := &OrgClient{
		orgID:  orgID,
		conn:   nc,
		js:     js,
		stream: stream,
	}

	// Insert under write lock (map write only, no I/O)
	p.mu.Lock()
	if _, exists := p.clients[orgID]; exists {
		p.mu.Unlock()
		nc.Close()
		return fmt.Errorf("org %s already in pool (race)", orgID)
	}
	p.clients[orgID] = client
	p.accountKeys[orgID] = accountKP
	p.mu.Unlock()

	// Audit log (after lock released)
	if p.auditLog != nil {
		p.auditLog.Log(ctx, "notifd", "credential.rotate", orgID, orgID, map[string]any{
			"user_pub_key":    userPub,
			"account_pub_key": accountPub,
		})
	}

	slog.Info("org connected to NATS", "org_id", orgID, "account_pub", accountPub)
	return nil
}

// Get returns the OrgClient for a given org ID.
func (p *ClientPool) Get(orgID string) (*OrgClient, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	client, ok := p.clients[orgID]
	if !ok {
		return nil, fmt.Errorf("no connection for org %s", orgID)
	}
	return client, nil
}

// Remove disconnects and removes an org from the pool.
func (p *ClientPool) Remove(orgID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	client, ok := p.clients[orgID]
	if !ok {
		return fmt.Errorf("no connection for org %s", orgID)
	}

	client.Close()
	delete(p.clients, orgID)
	delete(p.accountKeys, orgID)

	slog.Info("org disconnected from NATS", "org_id", orgID)
	return nil
}

// AccountKey returns the account key pair for an org (in-memory only).
func (p *ClientPool) AccountKey(orgID string) (nkeys.KeyPair, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	kp, ok := p.accountKeys[orgID]
	if !ok {
		return nil, fmt.Errorf("no account key for org %s", orgID)
	}
	return kp, nil
}

// SetAccountKey stores an account key pair for an org (in-memory only).
func (p *ClientPool) SetAccountKey(orgID string, kp nkeys.KeyPair) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.accountKeys[orgID] = kp
}

// OrgIDs returns a list of all connected org IDs.
func (p *ClientPool) OrgIDs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ids := make([]string, 0, len(p.clients))
	for id := range p.clients {
		ids = append(ids, id)
	}
	return ids
}

// OrgCount returns the number of connected orgs.
func (p *ClientPool) OrgCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.clients)
}

// IsHealthy returns true if the system connection and all org connections are active.
func (p *ClientPool) IsHealthy() bool {
	if p.system == nil || !p.system.IsConnected() {
		return false
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, client := range p.clients {
		if !client.IsConnected() {
			return false
		}
	}
	return true
}

// DisconnectedOrgs returns a list of org IDs with disconnected connections.
func (p *ClientPool) DisconnectedOrgs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var disconnected []string
	for orgID, client := range p.clients {
		if !client.IsConnected() {
			disconnected = append(disconnected, orgID)
		}
	}
	return disconnected
}

// Close drains all connections.
func (p *ClientPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, client := range p.clients {
		client.Close()
	}
	p.clients = make(map[string]*OrgClient)
	p.accountKeys = make(map[string]nkeys.KeyPair)

	if p.system != nil {
		p.system.Drain()
	}
}

// Boot connects all orgs from the database in parallel.
// HTTP server should block until this completes.
func (p *ClientPool) Boot(ctx context.Context, queries *db.Queries) error {
	orgs, err := queries.ListOrgs(ctx)
	if err != nil {
		return fmt.Errorf("list orgs: %w", err)
	}

	if len(orgs) == 0 {
		slog.Info("no orgs to connect")
		return nil
	}

	g, ctx := errgroup.WithContext(ctx)
	for _, org := range orgs {
		org := org
		g.Go(func() error {
			var accountKP nkeys.KeyPair

			// Restore account key pair from persisted seed
			if org.NatsAccountSeed.Valid && org.NatsAccountSeed.String != "" {
				kp, err := accounts.AccountKeyFromSeed(org.NatsAccountSeed.String)
				if err != nil {
					return fmt.Errorf("parse account seed for %s: %w", org.ID, err)
				}
				accountKP = kp
			} else {
				// Legacy org without seed — generate new and persist
				kp, err := accounts.GenerateAccountKey()
				if err != nil {
					return fmt.Errorf("generate account key for %s: %w", org.ID, err)
				}
				accountKP = kp

				newPub, err := accountKP.PublicKey()
				if err != nil {
					return fmt.Errorf("get public key for new account %s: %w", org.ID, err)
				}
				seedStr, err := accounts.Seed(accountKP)
				if err != nil {
					return fmt.Errorf("get seed for new account %s: %w", org.ID, err)
				}
				if err := queries.UpdateOrgNatsPublicKey(ctx, org.ID, newPub); err != nil {
					return fmt.Errorf("update nats public key for %s: %w", org.ID, err)
				}
				if err := queries.UpdateOrgNatsAccountSeed(ctx, org.ID, seedStr); err != nil {
					return fmt.Errorf("persist account seed for %s: %w", org.ID, err)
				}
				slog.Info("generated and persisted new account key", "org_id", org.ID)
			}

			// Push account JWT
			if err := p.jwtMgr.RebuildAndPushAccountJWT(ctx, org.ID, p.system); err != nil {
				return fmt.Errorf("push JWT for %s: %w", org.ID, err)
			}

			// Connect
			if err := p.Add(ctx, org.ID, accountKP); err != nil {
				return fmt.Errorf("add org %s to pool: %w", org.ID, err)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("boot orgs: %w", err)
	}

	slog.Info("all orgs connected", "count", len(orgs))
	return nil
}

// ensureStreamsForOrg creates the 3 per-account streams for an org.
func ensureStreamsForOrg(ctx context.Context, js jetstream.JetStream, orgID string) (jetstream.Stream, error) {
	// Main events stream
	stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        StreamName + "_" + orgID,
		Description: fmt.Sprintf("notif.sh events for org %s", orgID),
		Subjects:    []string{"events.>"},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      24 * time.Hour,
		MaxBytes:    1 << 30, // 1GB
		Replicas:    1,
		Discard:     jetstream.DiscardOld,
	})
	if err != nil {
		return nil, fmt.Errorf("create events stream for %s: %w", orgID, err)
	}
	slog.Info("JetStream stream ready", "name", StreamName+"_"+orgID)

	// Dead letter queue stream
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        DLQStreamName + "_" + orgID,
		Description: fmt.Sprintf("notif.sh DLQ for org %s", orgID),
		Subjects:    []string{"dlq.>"},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      7 * 24 * time.Hour,
		Replicas:    1,
	})
	if err != nil {
		return nil, fmt.Errorf("create DLQ stream for %s: %w", orgID, err)
	}
	slog.Info("JetStream stream ready", "name", DLQStreamName+"_"+orgID)

	// Webhook retry stream
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        WebhookRetryStream + "_" + orgID,
		Description: fmt.Sprintf("notif.sh webhook retry for org %s", orgID),
		Subjects:    []string{"webhook-retry.>"},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.WorkQueuePolicy,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})
	if err != nil {
		return nil, fmt.Errorf("create webhook retry stream for %s: %w", orgID, err)
	}
	slog.Info("JetStream stream ready", "name", WebhookRetryStream+"_"+orgID)

	return stream, nil
}

