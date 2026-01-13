package policy

import (
	"encoding/json"
	"fmt"
	"time"
)

// Enforcer checks permissions against policies
type Enforcer struct {
	loader  *Loader
	auditor *Auditor
}

// NewEnforcer creates a new policy enforcer
func NewEnforcer(loader *Loader, auditor *Auditor) *Enforcer {
	return &Enforcer{
		loader:  loader,
		auditor: auditor,
	}
}

// CheckPublish checks if a principal can publish to a topic
func (e *Enforcer) CheckPublish(principal Principal, topic string) CheckResult {
	result := e.check(principal, topic, PermissionPublish)

	// Audit the attempt
	if e.auditor != nil {
		e.auditor.Log(AuditEvent{
			Timestamp: time.Now(),
			OrgID:     principal.OrgID,
			Principal: principal,
			Action:    "publish",
			Topic:     topic,
			Result:    e.resultString(result.Allowed),
			Reason:    result.Reason,
			MatchedRule: result.MatchedRule,
		})
	}

	return result
}

// CheckSubscribe checks if a principal can subscribe to a topic
func (e *Enforcer) CheckSubscribe(principal Principal, topic string) CheckResult {
	result := e.check(principal, topic, PermissionSubscribe)

	// Audit the attempt
	if e.auditor != nil {
		e.auditor.Log(AuditEvent{
			Timestamp: time.Now(),
			OrgID:     principal.OrgID,
			Principal: principal,
			Action:    "subscribe",
			Topic:     topic,
			Result:    e.resultString(result.Allowed),
			Reason:    result.Reason,
			MatchedRule: result.MatchedRule,
		})
	}

	return result
}

// check performs the actual permission check
func (e *Enforcer) check(principal Principal, topic string, permission Permission) CheckResult {
	// Get policy for organization
	policy := e.loader.GetPolicy(principal.OrgID)
	if policy == nil {
		// No policy found - allow by default (backward compatibility)
		return CheckResult{
			Allowed: true,
			Reason:  "no policy found, default allow",
		}
	}

	// Find matching topic policies (most specific first)
	var matchedPolicies []*TopicPolicy
	for i := range policy.Topics {
		tp := &policy.Topics[i]
		if MatchTopic(tp.Pattern, topic) {
			matchedPolicies = append(matchedPolicies, tp)
		}
	}

	// If no topic policy matches, use default behavior
	if len(matchedPolicies) == 0 {
		if policy.DefaultDeny {
			return CheckResult{
				Allowed: false,
				Reason:  "no matching policy, default deny",
			}
		}
		return CheckResult{
			Allowed: true,
			Reason:  "no matching policy, default allow",
		}
	}

	// Check each matching policy (first match wins)
	for _, topicPolicy := range matchedPolicies {
		var rules []Rule
		if permission == PermissionPublish {
			rules = topicPolicy.Publish
		} else {
			rules = topicPolicy.Subscribe
		}

		// Check each rule
		for i := range rules {
			rule := &rules[i]
			if e.matchesRule(principal, rule) {
				return CheckResult{
					Allowed:       true,
					Reason:        fmt.Sprintf("matched policy for topic pattern %q", topicPolicy.Pattern),
					MatchedRule:   rule,
					MatchedPolicy: topicPolicy,
				}
			}
		}
	}

	// No rule matched - deny
	return CheckResult{
		Allowed: false,
		Reason:  fmt.Sprintf("no matching rule for topic %q", topic),
	}
}

// matchesRule checks if a principal matches a rule
func (e *Enforcer) matchesRule(principal Principal, rule *Rule) bool {
	// Check type if specified
	if rule.Type != "" {
		ruleType := PrincipalType(rule.Type)
		if ruleType != principal.Type {
			return false
		}
	}

	// Check identity pattern
	return MatchIdentity(rule.IdentityPattern, principal.ID)
}

func (e *Enforcer) resultString(allowed bool) string {
	if allowed {
		return "allowed"
	}
	return "denied"
}

// Auditor handles audit logging
type Auditor struct {
	publisher AuditPublisher
}

// AuditPublisher is an interface for publishing audit events
type AuditPublisher interface {
	PublishAudit(orgID string, event AuditEvent) error
}

// NewAuditor creates a new auditor
func NewAuditor(publisher AuditPublisher) *Auditor {
	return &Auditor{
		publisher: publisher,
	}
}

// Log records an audit event
func (a *Auditor) Log(event AuditEvent) {
	if a.publisher == nil {
		return
	}

	// Don't block on audit logging - fire and forget
	go func() {
		if err := a.publisher.PublishAudit(event.OrgID, event); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("ERROR: Failed to publish audit event: %v\n", err)
		}
	}()
}

// LogWithData logs an audit event with event data
func (a *Auditor) LogWithData(event AuditEvent, data interface{}) {
	event.EventData = data
	a.Log(event)
}

// FormatAuditEvent formats an audit event as JSON
func FormatAuditEvent(event AuditEvent) ([]byte, error) {
	return json.Marshal(event)
}
