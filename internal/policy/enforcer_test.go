package policy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnforcer(t *testing.T) {
	// Create temporary policy directory
	tmpDir := t.TempDir()

	// Write test policy
	policyContent := `
org_id: "test-org"
version: "1.0"
updated_at: 2024-01-13T00:00:00Z
default_deny: true
audit_enabled: false

topics:
  - pattern: "public.*"
    publish:
      - identity: "*"
    subscribe:
      - identity: "*"

  - pattern: "admin.*"
    publish:
      - identity: "admin-*"
        type: "api_key"
    subscribe:
      - identity: "admin-*"
        type: "api_key"

  - pattern: "service.notifications"
    publish:
      - identity: "service-*"
        type: "api_key"
    subscribe:
      - identity: "*"
`

	policyFile := filepath.Join(tmpDir, "test-org.yaml")
	if err := os.WriteFile(policyFile, []byte(policyContent), 0644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	// Create loader and enforcer
	loader, err := NewLoader(tmpDir)
	if err != nil {
		t.Fatalf("failed to create loader: %v", err)
	}
	defer loader.Close()

	enforcer := NewEnforcer(loader, nil) // No auditor for tests

	// Wait a bit for file to be loaded
	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name      string
		principal Principal
		topic     string
		action    string // "publish" or "subscribe"
		allowed   bool
	}{
		// Public topic - allowed for all
		{
			name: "public topic - api key can publish",
			principal: Principal{
				Type:  PrincipalAPIKey,
				ID:    "key-123",
				OrgID: "test-org",
			},
			topic:   "public.events",
			action:  "publish",
			allowed: true,
		},
		{
			name: "public topic - user can subscribe",
			principal: Principal{
				Type:  PrincipalUser,
				ID:    "user_123",
				OrgID: "test-org",
			},
			topic:   "public.notifications",
			action:  "subscribe",
			allowed: true,
		},

		// Admin topic - restricted to admin-* API keys
		{
			name: "admin topic - admin key can publish",
			principal: Principal{
				Type:  PrincipalAPIKey,
				ID:    "admin-service-1",
				OrgID: "test-org",
			},
			topic:   "admin.commands",
			action:  "publish",
			allowed: true,
		},
		{
			name: "admin topic - regular key cannot publish",
			principal: Principal{
				Type:  PrincipalAPIKey,
				ID:    "regular-key",
				OrgID: "test-org",
			},
			topic:   "admin.commands",
			action:  "publish",
			allowed: false,
		},
		{
			name: "admin topic - user cannot subscribe",
			principal: Principal{
				Type:  PrincipalUser,
				ID:    "user_123",
				OrgID: "test-org",
			},
			topic:   "admin.commands",
			action:  "subscribe",
			allowed: false,
		},

		// Service notifications - publish restricted, subscribe open
		{
			name: "service notifications - service key can publish",
			principal: Principal{
				Type:  PrincipalAPIKey,
				ID:    "service-backend",
				OrgID: "test-org",
			},
			topic:   "service.notifications",
			action:  "publish",
			allowed: true,
		},
		{
			name: "service notifications - regular key cannot publish",
			principal: Principal{
				Type:  PrincipalAPIKey,
				ID:    "regular-key",
				OrgID: "test-org",
			},
			topic:   "service.notifications",
			action:  "publish",
			allowed: false,
		},
		{
			name: "service notifications - anyone can subscribe",
			principal: Principal{
				Type:  PrincipalUser,
				ID:    "user_123",
				OrgID: "test-org",
			},
			topic:   "service.notifications",
			action:  "subscribe",
			allowed: true,
		},

		// Unknown topic with default_deny
		{
			name: "unknown topic - denied by default",
			principal: Principal{
				Type:  PrincipalAPIKey,
				ID:    "any-key",
				OrgID: "test-org",
			},
			topic:   "unknown.topic",
			action:  "publish",
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result CheckResult
			if tt.action == "publish" {
				result = enforcer.CheckPublish(tt.principal, tt.topic)
			} else {
				result = enforcer.CheckSubscribe(tt.principal, tt.topic)
			}

			if result.Allowed != tt.allowed {
				t.Errorf("got allowed=%v, want %v (reason: %s)", result.Allowed, tt.allowed, result.Reason)
			}
		})
	}
}

func TestEnforcerNoPolicyDefaultAllow(t *testing.T) {
	// Create empty temporary directory
	tmpDir := t.TempDir()

	loader, err := NewLoader(tmpDir)
	if err != nil {
		t.Fatalf("failed to create loader: %v", err)
	}
	defer loader.Close()

	enforcer := NewEnforcer(loader, nil)

	principal := Principal{
		Type:  PrincipalAPIKey,
		ID:    "any-key",
		OrgID: "nonexistent-org",
	}

	// When no policy exists, should allow by default (backward compatibility)
	result := enforcer.CheckPublish(principal, "any.topic")
	if !result.Allowed {
		t.Errorf("expected allow when no policy exists, got denied: %s", result.Reason)
	}
}
