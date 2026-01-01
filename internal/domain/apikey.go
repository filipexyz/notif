package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"time"

	"github.com/google/uuid"
)

type APIKey struct {
	ID                 uuid.UUID
	KeyPrefix          string
	Name               string
	RateLimitPerSecond int
	CreatedAt          time.Time
	LastUsedAt         *time.Time
	RevokedAt          *time.Time
	OrgID              string
}

// keyRegex matches: nsh_[a-zA-Z0-9]{20}
var keyRegex = regexp.MustCompile(`^nsh_[a-zA-Z0-9]{20}$`)

// ValidateKeyFormat checks if the key matches the expected format.
func ValidateKeyFormat(key string) bool {
	return keyRegex.MatchString(key)
}

// HashKey returns the SHA-256 hash of the key.
func HashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// GenerateAPIKey creates a new API key.
// Returns: full key, prefix (for display), hash (for storage)
func GenerateAPIKey() (fullKey string, prefix string, hash string) {
	// Generate 20 random alphanumeric characters
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 20)
	rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}

	fullKey = "nsh_" + string(b)
	prefix = fullKey[:12] // "nsh_abc12345"
	hash = HashKey(fullKey)
	return
}
