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
	// AuthModeNone disables Clerk auth, uses only API keys. Good for self-hosting.
	AuthModeNone AuthMode = "none"
)

type Config struct {
	// Server
	Port            string        `env:"PORT" envDefault:"8080"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"30s"`

	// Database
	DatabaseURL string `env:"DATABASE_URL,required"`

	// NATS
	NatsURL string `env:"NATS_URL" envDefault:"nats://localhost:4222"`

	// Logging
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	// Authentication
	// AUTH_MODE: "clerk" (default) or "none" (self-hosted, API keys only)
	AuthMode       AuthMode `env:"AUTH_MODE" envDefault:"clerk"`
	ClerkSecretKey string   `env:"CLERK_SECRET_KEY"`

	// Self-hosted mode settings (used when AUTH_MODE=none)
	// Default org ID for self-hosted single-tenant mode
	DefaultOrgID string `env:"DEFAULT_ORG_ID" envDefault:"org_default"`

	// CORS
	CORSOrigins []string `env:"CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000,http://localhost:5173"`

	// Terminal
	CLIBinaryPath string `env:"CLI_BINARY_PATH" envDefault:"/app/notif"`
}

// IsSelfHosted returns true if running in self-hosted mode (no Clerk).
func (c *Config) IsSelfHosted() bool {
	return c.AuthMode == AuthModeNone
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
