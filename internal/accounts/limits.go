package accounts

import (
	"time"

	"github.com/nats-io/jwt/v2"
)

// TierLimits defines NATS resource limits for a billing tier.
type TierLimits struct {
	MaxConnections int64
	MaxData        int64 // bytes
	MaxPayload     int64 // bytes per message
	MaxExports     int64
	MaxImports     int64
	StreamMaxAge   time.Duration
	StreamMaxBytes int64
}

// DefaultTierLimits returns the NATS limits for a given billing tier.
func DefaultTierLimits(tier string) TierLimits {
	switch tier {
	case "enterprise":
		return TierLimits{
			MaxConnections: 1000,
			MaxData:        -1, // unlimited
			MaxPayload:     1 << 20, // 1MB
			MaxExports:     -1,
			MaxImports:     -1,
			StreamMaxAge:   7 * 24 * time.Hour,
			StreamMaxBytes: 10 << 30, // 10GB
		}
	case "pro":
		return TierLimits{
			MaxConnections: 100,
			MaxData:        10 << 30, // 10GB
			MaxPayload:     1 << 20,  // 1MB
			MaxExports:     50,
			MaxImports:     50,
			StreamMaxAge:   24 * time.Hour,
			StreamMaxBytes: 1 << 30, // 1GB
		}
	default: // "free"
		return TierLimits{
			MaxConnections: 10,
			MaxData:        1 << 30,  // 1GB
			MaxPayload:     256 << 10, // 256KB
			MaxExports:     5,
			MaxImports:     5,
			StreamMaxAge:   12 * time.Hour,
			StreamMaxBytes: 256 << 20, // 256MB
		}
	}
}

// ValidTiers contains the set of known billing tiers.
var ValidTiers = map[string]bool{
	"free":       true,
	"pro":        true,
	"enterprise": true,
}

// IsValidTier returns true if the tier name is recognized.
func IsValidTier(tier string) bool {
	return ValidTiers[tier]
}

// ApplyLimits sets account JWT claims limits from a TierLimits.
func ApplyLimits(claims *jwt.AccountClaims, limits TierLimits) {
	claims.Limits.Conn = limits.MaxConnections
	claims.Limits.Data = limits.MaxData
	claims.Limits.Payload = limits.MaxPayload
	claims.Limits.Exports = limits.MaxExports
	claims.Limits.Imports = limits.MaxImports
}
