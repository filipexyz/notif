package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	// TestAPIKey is the API key used for testing (nsh_ + 28 chars)
	TestAPIKey = "nsh_abcdefghij1234567890abcdefgh"
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
		MaxPayloadSize:  262144, // 256KB
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
	// Find migrations directory (relative to test file location)
	migrationsDir := filepath.Join("..", "..", "db", "migrations")

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Filter and sort SQL files
	var migrationFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			migrationFiles = append(migrationFiles, entry.Name())
		}
	}
	sort.Strings(migrationFiles)

	// Execute each migration in order
	for _, file := range migrationFiles {
		content, err := os.ReadFile(filepath.Join(migrationsDir, file))
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file, err)
		}

		// Extract only the Up section (between "-- +goose Up" and "-- +goose Down")
		sql := extractGooseUp(string(content))

		if _, err := db.Exec(ctx, sql); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file, err)
		}
	}

	return nil
}

// extractGooseUp extracts the SQL between "-- +goose Up" and "-- +goose Down"
func extractGooseUp(content string) string {
	upMarker := "-- +goose Up"
	downMarker := "-- +goose Down"

	upIdx := strings.Index(content, upMarker)
	if upIdx == -1 {
		return content // No goose markers, return as-is
	}

	// Start after the Up marker
	start := upIdx + len(upMarker)

	// Find Down marker
	downIdx := strings.Index(content[start:], downMarker)
	if downIdx == -1 {
		return content[start:] // No Down section
	}

	return content[start : start+downIdx]
}

const (
	// TestOrgID is the organization ID used for testing
	TestOrgID = "org_test"
	// TestProjectID is the project ID used for testing
	TestProjectID = "prj_test123456789012345678901"
)

func seedTestAPIKey(ctx context.Context, db *pgxpool.Pool) error {
	// Create default project for test org
	_, err := db.Exec(ctx, `
		INSERT INTO projects (id, org_id, name, slug, created_at, updated_at)
		VALUES ($1, $2, 'Default', 'default', NOW(), NOW())
		ON CONFLICT (org_id, slug) DO NOTHING
	`, TestProjectID, TestOrgID)
	if err != nil {
		return fmt.Errorf("failed to create test project: %w", err)
	}

	// Create API key linked to project
	h := sha256.Sum256([]byte(TestAPIKey))
	hash := hex.EncodeToString(h[:])

	_, err = db.Exec(ctx, `
		INSERT INTO api_keys (key_hash, key_prefix, name, org_id, project_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (key_hash) DO NOTHING
	`, hash, "nsh_abcdefghi", "E2E Test Key", TestOrgID, TestProjectID)

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
