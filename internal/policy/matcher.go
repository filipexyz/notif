package policy

import (
	"strings"
)

// MatchTopic checks if a topic matches a pattern (supports wildcards)
// Patterns:
//   - "user.created" - exact match
//   - "user.*" - matches one segment: "user.created" but not "user.profile.updated"
//   - "user.>" - matches multiple segments: "user.created" and "user.profile.updated"
//   - "*" - matches any single segment topic
func MatchTopic(pattern, topic string) bool {
	// Exact match
	if pattern == topic {
		return true
	}

	// Empty pattern only matches empty topic
	if pattern == "" {
		return topic == ""
	}

	// Split into segments
	patternParts := strings.Split(pattern, ".")
	topicParts := strings.Split(topic, ".")

	return matchParts(patternParts, topicParts)
}

func matchParts(patternParts, topicParts []string) bool {
	pi, ti := 0, 0

	for pi < len(patternParts) {
		if ti >= len(topicParts) {
			// No more topic parts to match
			// Check if remaining pattern is only ">"
			if pi == len(patternParts)-1 && patternParts[pi] == ">" {
				// "user.>" should NOT match "user" - needs at least one more segment
				return false
			}
			return false
		}

		pPart := patternParts[pi]
		tPart := topicParts[ti]

		if pPart == ">" {
			// Multi-segment wildcard - matches rest of topic (at least one segment required)
			if ti >= len(topicParts) {
				// No segments left for > to match
				return false
			}
			// Consume all remaining topic parts
			return true
		} else if pPart == "*" {
			// Single-segment wildcard - matches exactly one segment
			pi++
			ti++
		} else if pPart == tPart {
			// Exact match
			pi++
			ti++
		} else {
			// No match
			return false
		}
	}

	// All pattern parts matched, check if all topic parts consumed
	return ti == len(topicParts)
}

// MatchIdentity checks if a principal ID matches an identity pattern
// Patterns:
//   - "abc123" - exact match
//   - "worker-*" - prefix match (matches "worker-", "worker-1", etc.)
//   - "*-prod" - suffix match (matches "-prod", "api-prod", etc.)
//   - "*" - matches any identity
func MatchIdentity(pattern, id string) bool {
	// Wildcard matches everything
	if pattern == "*" {
		return true
	}

	// Exact match
	if pattern == id {
		return true
	}

	// Prefix wildcard: "worker-*" matches "worker-", "worker-1", "worker-2", etc.
	// The part before * must be a prefix of id
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		// Check if id starts with the prefix
		// This handles both "worker-1" and "worker" (when prefix is "worker-")
		return strings.HasPrefix(id, prefix)
	}

	// Suffix wildcard: "*-prod" matches "-prod", "service-1-prod", "api-prod", etc.
	// The part after * must be a suffix of id
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		// Check if id ends with the suffix
		return strings.HasSuffix(id, suffix)
	}

	return false
}
