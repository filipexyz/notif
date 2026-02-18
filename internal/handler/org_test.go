package handler

import "testing"

func TestValidOrgID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"acme", true},
		{"my-org", true},
		{"my_org_123", true},
		{"UPPER", true},
		{"a", true},
		{"abcdefghijklmnopqrstuvwxyz012345", true},  // 32 chars
		{"abcdefghijklmnopqrstuvwxyz0123456", false}, // 33 chars
		{"", false},
		{"has space", false},
		{"has.dot", false},
		{"has/slash", false},
		{"has\nnewline", false},
		{"café", false},
		{"../traversal", false},
		{"org>inject", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := validOrgID.MatchString(tt.id)
			if got != tt.valid {
				t.Fatalf("validOrgID.MatchString(%q) = %v, want %v", tt.id, got, tt.valid)
			}
		})
	}
}
