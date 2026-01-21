package schema

import (
	"strings"
)

// MatchTopic checks if a topic matches a pattern.
// Patterns support:
//   - Exact match: "orders.placed" matches "orders.placed"
//   - Single wildcard: "orders.*" matches "orders.placed", "orders.shipped"
//   - Multi-level wildcard: "orders.>" matches "orders.placed", "orders.us.placed"
func MatchTopic(pattern, topic string) bool {
	// Exact match
	if pattern == topic {
		return true
	}

	patternParts := strings.Split(pattern, ".")
	topicParts := strings.Split(topic, ".")

	return matchParts(patternParts, topicParts)
}

func matchParts(pattern, topic []string) bool {
	pi, ti := 0, 0

	for pi < len(pattern) && ti < len(topic) {
		p := pattern[pi]

		switch p {
		case ">":
			// Multi-level wildcard - matches everything remaining
			return true
		case "*":
			// Single-level wildcard - matches one segment
			pi++
			ti++
		default:
			// Exact segment match
			if p != topic[ti] {
				return false
			}
			pi++
			ti++
		}
	}

	// Both must be exhausted for a match (unless pattern ends with >)
	return pi == len(pattern) && ti == len(topic)
}

// FindBestMatch finds the most specific matching pattern for a topic.
// More specific patterns (longer, fewer wildcards) are preferred.
func FindBestMatch(patterns []string, topic string) string {
	var bestMatch string
	bestScore := -1000000 // Start with very low score to allow negative specificity patterns

	for _, pattern := range patterns {
		if MatchTopic(pattern, topic) {
			score := patternSpecificity(pattern)
			if score > bestScore {
				bestScore = score
				bestMatch = pattern
			}
		}
	}

	return bestMatch
}

// patternSpecificity returns a score indicating how specific a pattern is.
// Higher scores = more specific.
func patternSpecificity(pattern string) int {
	parts := strings.Split(pattern, ".")
	score := len(parts) * 10 // Base score from length

	for _, p := range parts {
		switch p {
		case ">":
			score -= 100 // Multi-level wildcard is least specific
		case "*":
			score -= 5 // Single wildcard reduces specificity
		default:
			score += 1 // Exact segments add specificity
		}
	}

	return score
}

// ExpandWildcards returns all possible concrete topic prefixes from a pattern.
// This is useful for database queries.
// e.g., "orders.*" -> "orders."
// e.g., "orders.>" -> "orders."
// e.g., "orders.placed" -> "orders.placed"
func ExpandWildcards(pattern string) string {
	parts := strings.Split(pattern, ".")
	var result []string

	for _, p := range parts {
		if p == "*" || p == ">" {
			break
		}
		result = append(result, p)
	}

	if len(result) == 0 {
		return ""
	}

	return strings.Join(result, ".") + "."
}
