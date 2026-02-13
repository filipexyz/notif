// Package security provides security utilities for the notif server.
package security

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

var (
	ErrInvalidURL          = errors.New("invalid URL")
	ErrInvalidScheme       = errors.New("URL must use http or https scheme")
	ErrPrivateIP           = errors.New("URL points to a private IP address")
	ErrLoopbackIP          = errors.New("URL points to a loopback address")
	ErrLinkLocalIP         = errors.New("URL points to a link-local address")
	ErrMetadataEndpoint    = errors.New("URL points to a cloud metadata endpoint")
	ErrUnresolvableHost    = errors.New("cannot resolve hostname")
	ErrInvalidPort         = errors.New("URL contains an invalid or blocked port")
	ErrSuspiciousURL       = errors.New("URL contains suspicious patterns")
)

// blockedHosts contains hostnames that should never be allowed
var blockedHosts = map[string]bool{
	"metadata.google.internal":         true,
	"metadata.goog":                    true,
	"kubernetes.default.svc":           true,
	"kubernetes.default":               true,
	"localhost":                        true,
	"localhost.localdomain":            true,
	"localtest.me":                     true,
	"lvh.me":                           true,
	"vcap.me":                          true,
}

// blockedPorts contains ports commonly used for internal services
var blockedPorts = map[string]bool{
	"22":    true, // SSH
	"25":    true, // SMTP
	"6379":  true, // Redis
	"11211": true, // Memcached
	"27017": true, // MongoDB
	"3306":  true, // MySQL
	"5432":  true, // PostgreSQL
	"9200":  true, // Elasticsearch
	"2379":  true, // etcd
	"8500":  true, // Consul
}

// ValidateWebhookURL validates a webhook URL to prevent SSRF attacks.
// It checks for:
// - Valid URL format
// - Allowed schemes (http/https only)
// - No private/internal IP addresses
// - No cloud metadata endpoints
// - No localhost or loopback addresses
// - No suspicious URL patterns (encoding tricks, etc.)
func ValidateWebhookURL(rawURL string) error {
	// Basic URL parsing
	u, err := url.Parse(rawURL)
	if err != nil {
		return ErrInvalidURL
	}

	// Check scheme - only http and https allowed
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrInvalidScheme
	}

	// Check for suspicious URL patterns that might bypass validation
	if containsSuspiciousPatterns(rawURL) {
		return ErrSuspiciousURL
	}

	// Extract hostname (without port)
	hostname := u.Hostname()
	if hostname == "" {
		return ErrInvalidURL
	}

	// Normalize hostname
	hostname = strings.ToLower(hostname)

	// Check against blocked hostnames
	if blockedHosts[hostname] {
		return ErrMetadataEndpoint
	}

	// Check for localhost variations
	if isLocalhostVariation(hostname) {
		return ErrLoopbackIP
	}

	// Check port if specified
	if port := u.Port(); port != "" {
		if blockedPorts[port] {
			return ErrInvalidPort
		}
	}

	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return ErrUnresolvableHost
	}

	// Check all resolved IPs
	for _, ip := range ips {
		if err := ValidateIP(ip); err != nil {
			return err
		}
	}

	return nil
}

// ValidateIP checks if an IP address is safe to use.
func ValidateIP(ip net.IP) error {
	// Check for loopback (127.0.0.0/8, ::1)
	if ip.IsLoopback() {
		return ErrLoopbackIP
	}

	// Check for private networks
	// 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
	if ip.IsPrivate() {
		return ErrPrivateIP
	}

	// Check for link-local addresses
	// 169.254.0.0/16 (includes AWS metadata 169.254.169.254)
	// fe80::/10
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return ErrLinkLocalIP
	}

	// Check for unspecified address (0.0.0.0, ::)
	if ip.IsUnspecified() {
		return ErrPrivateIP
	}

	// Additional check for AWS/cloud metadata IP
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return ErrMetadataEndpoint
	}

	// Check for Alibaba Cloud metadata
	if ip.Equal(net.ParseIP("100.100.100.200")) {
		return ErrMetadataEndpoint
	}

	return nil
}

// containsSuspiciousPatterns checks for URL encoding tricks and bypass attempts
func containsSuspiciousPatterns(rawURL string) bool {
	lower := strings.ToLower(rawURL)

	// Check for various encoding bypass attempts
	suspiciousPatterns := []string{
		"%00",           // Null byte
		"%0d",           // CR
		"%0a",           // LF
		"\\",            // Backslash (path confusion)
		"@",             // Userinfo (user@host bypass)
		"#",             // Fragment before host
		"0x",            // Hex IP encoding (http://0x7f000001)
		"[:",            // IPv6 brackets
		"::ffff:",       // IPv4-mapped IPv6
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Check for decimal IP notation (http://2130706433)
	// A pure decimal number as hostname is suspicious
	hostname := extractHostname(rawURL)
	if hostname != "" && isDecimalIP(hostname) {
		return true
	}

	// Check for octal IP notation (http://017700000001)
	if hostname != "" && isOctalIP(hostname) {
		return true
	}

	return false
}

// extractHostname extracts hostname from URL for additional checks
func extractHostname(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// isDecimalIP checks if hostname is a decimal IP representation
func isDecimalIP(hostname string) bool {
	// Check if it's all digits (decimal IP like 2130706433)
	for _, c := range hostname {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(hostname) > 0
}

// isOctalIP checks if hostname looks like an octal IP
func isOctalIP(hostname string) bool {
	// Octal IPs start with 0 and contain only digits
	if !strings.HasPrefix(hostname, "0") {
		return false
	}
	for _, c := range hostname {
		if c < '0' || c > '7' {
			return false
		}
	}
	return len(hostname) > 1
}

// isLocalhostVariation checks for various localhost representations
func isLocalhostVariation(hostname string) bool {
	variations := []string{
		"localhost",
		"127.0.0.1",
		"127.0.1",
		"127.1",
		"0.0.0.0",
		"0",
		"[::1]",
		"::1",
	}

	for _, v := range variations {
		if hostname == v {
			return true
		}
	}

	// Check for localhost subdomains or any hostname containing "localhost"
	if strings.Contains(hostname, "localhost") {
		return true
	}

	// Check for common DNS rebinding domains
	rebindingDomains := []string{
		".nip.io",
		".xip.io",
		".sslip.io",
		".localtest.me",
		".lvh.me",
		".vcap.me",
		".lacolhost.com",
		".127-0-0-1.",
		".rebind.network",
	}

	for _, domain := range rebindingDomains {
		if strings.Contains(hostname, domain) || strings.HasSuffix(hostname, domain) {
			return true
		}
	}

	return false
}

