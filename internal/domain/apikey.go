package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Environment string

const (
	EnvLive Environment = "live"
	EnvTest Environment = "test"
)

type APIKey struct {
	ID                 uuid.UUID
	KeyPrefix          string
	Environment        Environment
	Name               string
	RateLimitPerSecond int
	CreatedAt          time.Time
	LastUsedAt         *time.Time
	RevokedAt          *time.Time
}

// keyRegex matches: nsh_(live|test)_[a-zA-Z0-9]{24}
var keyRegex = regexp.MustCompile(`^nsh_(live|test)_[a-zA-Z0-9]{24}$`)

// ValidateKeyFormat checks if the key matches the expected format.
func ValidateKeyFormat(key string) bool {
	return keyRegex.MatchString(key)
}

// ParseKeyEnvironment extracts the environment from a key.
func ParseKeyEnvironment(key string) (Environment, error) {
	if !ValidateKeyFormat(key) {
		return "", errors.New("invalid key format")
	}
	parts := strings.Split(key, "_")
	return Environment(parts[1]), nil
}

// HashKey returns the SHA-256 hash of the key.
func HashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// GenerateAPIKey creates a new API key with the given environment.
func GenerateAPIKey(env Environment) (fullKey string, prefix string, hash string) {
	// Generate 24 random alphanumeric characters
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 24)
	rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}

	fullKey = "nsh_" + string(env) + "_" + string(b)
	prefix = fullKey[:16] // "nsh_live_abc1234"
	hash = HashKey(fullKey)
	return
}
