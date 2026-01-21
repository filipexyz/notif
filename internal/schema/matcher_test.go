package schema

import (
	"testing"
)

func TestMatchTopic(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		topic   string
		want    bool
	}{
		// Exact matches
		{"exact match", "orders.placed", "orders.placed", true},
		{"exact no match", "orders.placed", "orders.shipped", false},
		{"exact no match different length", "orders.placed", "orders.placed.us", false},

		// Single wildcard (*)
		{"single wildcard match", "orders.*", "orders.placed", true},
		{"single wildcard match 2", "orders.*", "orders.shipped", true},
		{"single wildcard no match too deep", "orders.*", "orders.us.placed", false},
		{"single wildcard middle", "orders.*.confirmed", "orders.us.confirmed", true},
		{"single wildcard middle no match", "orders.*.confirmed", "orders.us.pending", false},
		{"multiple single wildcards", "*.orders.*", "us.orders.placed", true},

		// Multi-level wildcard (>)
		{"multi wildcard match one level", "orders.>", "orders.placed", true},
		{"multi wildcard match two levels", "orders.>", "orders.us.placed", true},
		{"multi wildcard match three levels", "orders.>", "orders.us.east.placed", true},
		{"multi wildcard no match prefix", "orders.>", "inventory.placed", false},

		// Mixed wildcards
		{"mixed wildcards", "orders.*.>", "orders.us.placed", true},
		{"mixed wildcards deep", "orders.*.>", "orders.us.east.placed", true},
		{"mixed wildcards no match", "orders.*.>", "orders.placed", false},

		// Edge cases
		{"empty pattern empty topic", "", "", true},
		{"single segment", "orders", "orders", true},
		{"single segment no match", "orders", "inventory", false},
		{"wildcard only", "*", "orders", true},
		{"multi wildcard only", ">", "orders", true},
		{"multi wildcard only deep", ">", "orders.placed.us", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchTopic(tt.pattern, tt.topic)
			if got != tt.want {
				t.Errorf("MatchTopic(%q, %q) = %v, want %v", tt.pattern, tt.topic, got, tt.want)
			}
		})
	}
}

func TestFindBestMatch(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		topic    string
		want     string
	}{
		{
			name:     "exact match preferred over wildcard",
			patterns: []string{"orders.*", "orders.placed"},
			topic:    "orders.placed",
			want:     "orders.placed",
		},
		{
			name:     "single wildcard preferred over multi",
			patterns: []string{"orders.>", "orders.*"},
			topic:    "orders.placed",
			want:     "orders.*",
		},
		{
			name:     "longer pattern preferred",
			patterns: []string{"orders.*", "orders.us.*"},
			topic:    "orders.us.placed",
			want:     "orders.us.*",
		},
		{
			name:     "no match returns empty",
			patterns: []string{"orders.*", "inventory.*"},
			topic:    "users.created",
			want:     "",
		},
		{
			name:     "multi wildcard matches deep topics",
			patterns: []string{"orders.>"},
			topic:    "orders.us.east.placed",
			want:     "orders.>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindBestMatch(tt.patterns, tt.topic)
			if got != tt.want {
				t.Errorf("FindBestMatch(%v, %q) = %q, want %q", tt.patterns, tt.topic, got, tt.want)
			}
		})
	}
}

func TestPatternSpecificity(t *testing.T) {
	tests := []struct {
		pattern1 string
		pattern2 string
		wantMore string // which pattern should have higher specificity
	}{
		{"orders.placed", "orders.*", "orders.placed"},
		{"orders.*", "orders.>", "orders.*"},
		{"orders.us.*", "orders.*", "orders.us.*"},
		{"orders.us.placed", "orders.*.placed", "orders.us.placed"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern1+" vs "+tt.pattern2, func(t *testing.T) {
			score1 := patternSpecificity(tt.pattern1)
			score2 := patternSpecificity(tt.pattern2)

			if tt.wantMore == tt.pattern1 && score1 <= score2 {
				t.Errorf("patternSpecificity(%q)=%d should be > patternSpecificity(%q)=%d",
					tt.pattern1, score1, tt.pattern2, score2)
			}
			if tt.wantMore == tt.pattern2 && score2 <= score1 {
				t.Errorf("patternSpecificity(%q)=%d should be > patternSpecificity(%q)=%d",
					tt.pattern2, score2, tt.pattern1, score1)
			}
		})
	}
}

func TestExpandWildcards(t *testing.T) {
	tests := []struct {
		pattern string
		want    string
	}{
		{"orders.*", "orders."},
		{"orders.>", "orders."},
		{"orders.placed", "orders.placed."},
		{"orders.us.*", "orders.us."},
		{"*", ""},
		{">", ""},
		{"orders.*.placed", "orders."},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := ExpandWildcards(tt.pattern)
			if got != tt.want {
				t.Errorf("ExpandWildcards(%q) = %q, want %q", tt.pattern, got, tt.want)
			}
		})
	}
}
