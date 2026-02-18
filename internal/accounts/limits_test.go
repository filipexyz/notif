package accounts

import "testing"

func TestIsValidTier(t *testing.T) {
	tests := []struct {
		tier  string
		valid bool
	}{
		{"free", true},
		{"pro", true},
		{"enterprise", true},
		{"", false},
		{"platinum", false},
		{"FREE", false},
		{"Pro", false},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			if got := IsValidTier(tt.tier); got != tt.valid {
				t.Fatalf("IsValidTier(%q) = %v, want %v", tt.tier, got, tt.valid)
			}
		})
	}
}

func TestDefaultTierLimits(t *testing.T) {
	// Ensure all valid tiers return non-zero connection limits
	for tier := range ValidTiers {
		limits := DefaultTierLimits(tier)
		if limits.MaxConnections <= 0 {
			t.Fatalf("tier %q has MaxConnections %d", tier, limits.MaxConnections)
		}
	}

	// Unknown tier falls back to free
	unknown := DefaultTierLimits("garbage")
	free := DefaultTierLimits("free")
	if unknown.MaxConnections != free.MaxConnections {
		t.Fatal("unknown tier should fall back to free limits")
	}
}
