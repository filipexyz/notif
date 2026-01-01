package cmd

import "testing"

func TestVersionString(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		commit   string
		expected string
	}{
		{
			name:     "dev version shows commit",
			version:  "dev",
			commit:   "abc1234",
			expected: "notif dev (abc1234)",
		},
		{
			name:     "release version has v prefix",
			version:  "0.1.2",
			commit:   "abc1234",
			expected: "notif v0.1.2",
		},
		{
			name:     "pre-release version",
			version:  "1.0.0-beta.1",
			commit:   "def5678",
			expected: "notif v1.0.0-beta.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore
			origVersion, origCommit := Version, Commit
			defer func() {
				Version, Commit = origVersion, origCommit
			}()

			Version = tt.version
			Commit = tt.commit

			if got := VersionString(); got != tt.expected {
				t.Errorf("VersionString() = %q, want %q", got, tt.expected)
			}
		})
	}
}
