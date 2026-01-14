package config

import (
	"time"

	"github.com/caarlos0/env/v10"
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

	// Clerk Authentication (for dashboard routes)
	ClerkSecretKey string `env:"CLERK_SECRET_KEY"`

	// CORS
	CORSOrigins []string `env:"CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000,http://localhost:5173"`

	// Terminal
	CLIBinaryPath string `env:"CLI_BINARY_PATH" envDefault:"notif"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
