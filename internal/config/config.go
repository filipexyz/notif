package config

import (
	"time"

	"github.com/caarlos0/env/v10"
)

// AuthMode determines the authentication mode for the server.
type AuthMode string

const (
	// AuthModeClerk uses Clerk for dashboard auth + API keys for API access.
	AuthModeClerk AuthMode = "clerk"
	// AuthModeLocal uses only API keys (no external auth provider). For self-hosting.
	AuthModeLocal AuthMode = "local"
)

type Config struct {
	// Server
	Port            string        `env:"PORT" envDefault:"8080"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"30s"`
	MaxPayloadSize  int64         `env:"MAX_PAYLOAD_SIZE" envDefault:"262144"` // 256KB

	// Database
	DatabaseURL string `env:"DATABASE_URL,required"`

	// NATS
	NatsURL            string `env:"NATS_URL" envDefault:"nats://localhost:4222"`
	OperatorSeed       string `env:"OPERATOR_SEED"`        // NATS operator NKey seed (required for multi-account)
	SystemAccountSeed  string `env:"SYSTEM_ACCOUNT_SEED"`  // NATS system account NKey seed (required for multi-account)

	// Embedded NATS server (optional — starts NATS in-process)
	NatsEmbedded bool   `env:"NATS_EMBEDDED" envDefault:"false"`
	NatsStoreDir string `env:"NATS_STORE_DIR" envDefault:"/data/nats"`

	// MultiAccount enables NATS multi-account isolation.
	// When false, uses legacy single-connection mode.
	MultiAccount bool `env:"NATS_MULTI_ACCOUNT" envDefault:"false"`

	// Logging
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	// Authentication
	// AUTH_MODE: "clerk" (default) or "local" (self-hosted, API keys only)
	AuthMode       AuthMode `env:"AUTH_MODE" envDefault:"clerk"`
	ClerkSecretKey string   `env:"CLERK_SECRET_KEY"`

	// Self-hosted mode settings (used when AUTH_MODE=local)
	// Default org ID for self-hosted single-tenant mode
	DefaultOrgID string `env:"DEFAULT_ORG_ID" envDefault:"org_default"`

	// CORS
	CORSOrigins []string `env:"CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000,http://localhost:5173"`

	// Interceptors & Federation (optional)
	InterceptorsConfigPath string `env:"INTERCEPTORS_CONFIG" envDefault:""`
	FederationConfigPath   string `env:"FEDERATION_CONFIG" envDefault:""`

	// Terminal
	CLIBinaryPath string `env:"CLI_BINARY_PATH" envDefault:"/app/notif"`
}

// IsSelfHosted returns true if running in self-hosted mode (no Clerk).
func (c *Config) IsSelfHosted() bool {
	return c.AuthMode == AuthModeLocal
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
