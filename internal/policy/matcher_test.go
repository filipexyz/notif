package policy

import (
	"testing"
)

func TestMatchTopic(t *testing.T) {
	tests := []struct {
		pattern  string
		topic    string
		expected bool
	}{
		// Exact matches
		{"user.created", "user.created", true},
		{"user.created", "user.updated", false},

		// Single-segment wildcard (*)
		{"user.*", "user.created", true},
		{"user.*", "user.updated", true},
		{"user.*", "user", false},
		{"user.*", "user.profile.updated", false}, // * doesn't match multiple segments

		// Multi-segment wildcard (>)
		{"user.>", "user.created", true},
		{"user.>", "user.profile.updated", true},
		{"user.>", "user.profile.picture.uploaded", true},
		{"user.>", "user", false},
		{"admin.>", "user.created", false},

		// Empty cases
		{"", "", true},
		{"", "user.created", false},
		{"user.created", "", false},

		// Complex patterns
		{"*.created", "user.created", true},
		{"*.created", "order.created", true},
		{"*.created", "user.updated", false},
		{"*.created", "user.profile.created", false},

		// Wildcards at end
		{"user.*", "user.a", true},
		{"user.>", "user.a.b.c", true},
	}

	for _, tt := range tests {
		result := MatchTopic(tt.pattern, tt.topic)
		if result != tt.expected {
			t.Errorf("MatchTopic(%q, %q) = %v, want %v", tt.pattern, tt.topic, result, tt.expected)
		}
	}
}

func TestMatchIdentity(t *testing.T) {
	tests := []struct {
		pattern  string
		id       string
		expected bool
	}{
		// Exact matches
		{"user_123", "user_123", true},
		{"user_123", "user_456", false},

		// Wildcard matches all
		{"*", "user_123", true},
		{"*", "anything", true},
		{"*", "", true},

		// Prefix wildcard
		{"worker-*", "worker-1", true},
		{"worker-*", "worker-2", true},
		{"worker-*", "worker-abc", true},
		{"worker-*", "worker-", true}, // Just the prefix
		{"worker-*", "service-1", false},
		{"worker-*", "worker", false}, // Doesn't include the dash

		// Suffix wildcard
		{"*-prod", "api-prod", true},
		{"*-prod", "service-1-prod", true},
		{"*-prod", "-prod", true}, // Just the suffix
		{"*-prod", "prod", false}, // Doesn't include the dash
		{"*-prod", "api-dev", false},

		// No wildcard - must match exactly
		{"user_123", "user_123", true},
		{"user_123", "user_124", false},
	}

	for _, tt := range tests {
		result := MatchIdentity(tt.pattern, tt.id)
		if result != tt.expected {
			t.Errorf("MatchIdentity(%q, %q) = %v, want %v", tt.pattern, tt.id, result, tt.expected)
		}
	}
}
