package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/filipexyz/notif/internal/config"
	notifnats "github.com/filipexyz/notif/internal/nats"
	"github.com/filipexyz/notif/internal/server"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// TestAPIKey is the API key used for testing (24 chars after prefix)
	TestAPIKey = "nsh_test_abcdefghij12345678901234"
)

// TestEnv holds all test dependencies
type TestEnv struct {
	DB        *pgxpool.Pool
	NATS      *notifnats.Client
	Server    *server.Server
	ServerURL string
	PostgresC testcontainers.Container
	NATSC     testcontainers.Container
	cancel    context.CancelFunc
}

// SetupTestEnv creates a complete test environment with containers
func SetupTestEnv(t *testing.T) *TestEnv {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

	env := &TestEnv{
		cancel: cancel,
	}

	// Start Postgres container
	postgresC, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("notif_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}
	env.PostgresC = postgresC

	// Start NATS container with JetStream using generic container
	natsC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2.10-alpine",
			ExposedPorts: []string{"4222/tcp"},
			Cmd:          []string{"-js"}, // Enable JetStream
			WaitingFor:   wait.ForListeningPort("4222/tcp").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("failed to start nats: %v", err)
	}
	env.NATSC = natsC

	// Get connection strings
	postgresURL, err := postgresC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get postgres connection string: %v", err)
	}

	natsHost, err := natsC.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get nats host: %v", err)
	}
	natsPort, err := natsC.MappedPort(ctx, "4222")
	if err != nil {
		t.Fatalf("failed to get nats port: %v", err)
	}
	natsURL := fmt.Sprintf("nats://%s:%s", natsHost, natsPort.Port())

	// Connect to Postgres
	db, err := pgxpool.New(ctx, postgresURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	env.DB = db

	// Run migrations
	if err := runMigrations(ctx, db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Seed test API key
	if err := seedTestAPIKey(ctx, db); err != nil {
		t.Fatalf("failed to seed test API key: %v", err)
	}

	// Connect to NATS
	nc, err := notifnats.Connect(natsURL)
	if err != nil {
		t.Fatalf("failed to connect to nats: %v", err)
	}
	env.NATS = nc

	// Ensure streams
	if err := nc.EnsureStreams(ctx); err != nil {
		t.Fatalf("failed to ensure streams: %v", err)
	}

	// Create server
	cfg := &config.Config{
		Port:            "0",
		ShutdownTimeout: 5 * time.Second,
		DatabaseURL:     postgresURL,
		NatsURL:         natsURL,
		LogLevel:        "debug",
		LogFormat:       "text",
	}

	srv := server.New(cfg, db, nc)

	// Start server on random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	env.ServerURL = fmt.Sprintf("http://%s", listener.Addr().String())

	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			t.Logf("server error: %v", err)
		}
	}()

	env.Server = srv

	// Wait for server to be ready
	if err := waitForServer(env.ServerURL); err != nil {
		t.Fatalf("server not ready: %v", err)
	}

	return env
}

// Cleanup tears down the test environment
func (e *TestEnv) Cleanup(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if e.Server != nil {
		e.Server.Shutdown(ctx)
	}
	if e.NATS != nil {
		e.NATS.Close()
	}
	if e.DB != nil {
		e.DB.Close()
	}
	if e.NATSC != nil {
		e.NATSC.Terminate(ctx)
	}
	if e.PostgresC != nil {
		e.PostgresC.Terminate(ctx)
	}
	e.cancel()
}

func runMigrations(ctx context.Context, db *pgxpool.Pool) error {
	migration := `
		CREATE TABLE IF NOT EXISTS api_keys (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			key_hash VARCHAR(64) NOT NULL UNIQUE,
			key_prefix VARCHAR(32) NOT NULL,
			environment VARCHAR(10) NOT NULL CHECK (environment IN ('live', 'test')),
			name VARCHAR(255),
			rate_limit_per_second INT DEFAULT 100,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_used_at TIMESTAMPTZ,
			revoked_at TIMESTAMPTZ
		);

		CREATE TABLE IF NOT EXISTS events (
			id VARCHAR(32) PRIMARY KEY,
			topic VARCHAR(255) NOT NULL,
			api_key_id UUID REFERENCES api_keys(id),
			environment VARCHAR(10) NOT NULL,
			payload_size INT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS consumer_groups (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			api_key_id UUID REFERENCES api_keys(id),
			environment VARCHAR(10) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(name, api_key_id)
		);
	`
	_, err := db.Exec(ctx, migration)
	return err
}

func seedTestAPIKey(ctx context.Context, db *pgxpool.Pool) error {
	h := sha256.Sum256([]byte(TestAPIKey))
	hash := hex.EncodeToString(h[:])

	_, err := db.Exec(ctx, `
		INSERT INTO api_keys (key_hash, key_prefix, environment, name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (key_hash) DO NOTHING
	`, hash, "nsh_test_abcdef", "test", "E2E Test Key")

	return err
}

func waitForServer(url string) error {
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server not ready after 10s")
}
