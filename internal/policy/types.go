package policy

import (
	"time"
)

// Permission represents what operations are allowed
type Permission string

const (
	PermissionPublish   Permission = "publish"
	PermissionSubscribe Permission = "subscribe"
)

// PrincipalType represents who/what is being granted access
type PrincipalType string

const (
	PrincipalAPIKey PrincipalType = "api_key"
	PrincipalUser   PrincipalType = "user"
)

// Principal represents an authenticated entity
type Principal struct {
	Type  PrincipalType
	ID    string // API key ID or user ID
	OrgID string
}

// Rule represents a single access control rule
type Rule struct {
	// Principal identification
	IdentityPattern string   `yaml:"identity"` // e.g., "api-key-*", "user-123", "*"
	Type            string   `yaml:"type"`     // "api_key" or "user"
	Permissions     []string `yaml:"permissions"`

	// Optional constraints
	Description string `yaml:"description,omitempty"`
}

// TopicPolicy defines access rules for a topic pattern
type TopicPolicy struct {
	// Topic pattern (supports * and >)
	Pattern string `yaml:"pattern"`

	// Rules for publishing
	Publish []Rule `yaml:"publish,omitempty"`

	// Rules for subscribing
	Subscribe []Rule `yaml:"subscribe,omitempty"`

	// Policy metadata
	Description string `yaml:"description,omitempty"`
	CreatedAt   string `yaml:"created_at,omitempty"`
}

// OrgPolicy represents the complete policy file for an organization
type OrgPolicy struct {
	// Organization identifier
	OrgID string `yaml:"org_id"`

	// Policy metadata
	Version     string    `yaml:"version"`
	Description string    `yaml:"description,omitempty"`
	UpdatedAt   time.Time `yaml:"updated_at"`

	// Default policy when no specific rules match
	DefaultDeny bool `yaml:"default_deny"` // If true, deny unless explicitly allowed

	// Topic-level policies
	Topics []TopicPolicy `yaml:"topics"`

	// Audit settings
	AuditEnabled      bool   `yaml:"audit_enabled"`
	AuditTopic        string `yaml:"audit_topic,omitempty"`
	AuditDeniedOnly   bool   `yaml:"audit_denied_only"` // Only log denied attempts
	AuditIncludeData  bool   `yaml:"audit_include_data"` // Include event data in audit logs
}

// CheckResult represents the outcome of a permission check
type CheckResult struct {
	Allowed       bool
	Reason        string // Why allowed or denied
	MatchedRule   *Rule
	MatchedPolicy *TopicPolicy
}

// AuditEvent represents a security audit log entry
type AuditEvent struct {
	Timestamp   time.Time     `json:"timestamp"`
	OrgID       string        `json:"org_id"`
	Principal   Principal     `json:"principal"`
	Action      string        `json:"action"` // "publish" or "subscribe"
	Topic       string        `json:"topic"`
	Result      string        `json:"result"` // "allowed" or "denied"
	Reason      string        `json:"reason"`
	MatchedRule *Rule         `json:"matched_rule,omitempty"`
	EventData   interface{}   `json:"event_data,omitempty"` // Optional, based on policy
}
