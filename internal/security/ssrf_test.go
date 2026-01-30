package security

import (
	"net"
	"testing"
)

func TestValidateWebhookURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errType error
	}{
		// Valid URLs
		{"valid https", "https://example.com/webhook", false, nil},
		{"valid http", "http://example.com/webhook", false, nil},
		{"valid with port", "https://example.com:8080/webhook", false, nil},
		{"valid with path", "https://api.example.com/v1/webhooks/callback", false, nil},

		// Invalid schemes
		{"gopher protocol", "gopher://127.0.0.1:6379/_INFO", true, ErrInvalidScheme},
		{"file protocol", "file:///etc/passwd", true, ErrInvalidScheme},
		{"ftp protocol", "ftp://example.com/file", true, ErrInvalidScheme},
		{"dict protocol", "dict://127.0.0.1:6379/INFO", true, ErrInvalidScheme},

		// Localhost variations
		{"localhost", "http://localhost/callback", true, ErrMetadataEndpoint}, // In blockedHosts
		{"127.0.0.1", "http://127.0.0.1/callback", true, ErrLoopbackIP},
		{"127.1", "http://127.1/callback", true, ErrLoopbackIP},
		{"0.0.0.0", "http://0.0.0.0/callback", true, ErrLoopbackIP},

		// Private IPs
		{"10.x.x.x", "http://10.0.0.1/callback", true, ErrPrivateIP},
		{"172.16.x.x", "http://172.16.0.1/callback", true, ErrPrivateIP},
		{"192.168.x.x", "http://192.168.1.1/callback", true, ErrPrivateIP},

		// Cloud metadata endpoints
		{"AWS metadata", "http://169.254.169.254/latest/meta-data/", true, ErrLinkLocalIP},
		{"GCP metadata", "http://metadata.google.internal/computeMetadata/v1/", true, ErrMetadataEndpoint},
		{"Alibaba metadata", "http://100.100.100.200/latest/meta-data/", true, ErrMetadataEndpoint},

		// URL bypass attempts
		{"hex IP", "http://0x7f000001/", true, ErrSuspiciousURL},
		{"decimal IP", "http://2130706433/", true, ErrSuspiciousURL},
		{"octal IP", "http://017700000001/", true, ErrSuspiciousURL},
		{"userinfo bypass", "http://evil.com@127.0.0.1/", true, ErrSuspiciousURL},
		{"IPv6 localhost", "http://[::1]/", true, ErrSuspiciousURL},
		{"IPv4 mapped IPv6", "http://[::ffff:127.0.0.1]/", true, ErrSuspiciousURL},

		// DNS rebinding domains
		{"nip.io", "http://127.0.0.1.nip.io/callback", true, ErrLoopbackIP},
		{"xip.io", "http://127.0.0.1.xip.io/callback", true, ErrLoopbackIP},
		{"localtest.me", "http://localtest.me/callback", true, ErrMetadataEndpoint}, // In blockedHosts

		// Blocked ports
		{"SSH port", "http://example.com:22/callback", true, ErrInvalidPort},
		{"Redis port", "http://example.com:6379/callback", true, ErrInvalidPort},
		{"MySQL port", "http://example.com:3306/callback", true, ErrInvalidPort},

		// Invalid URLs
		{"empty URL", "", true, ErrInvalidScheme},    // Empty string has no scheme
		{"no scheme", "example.com/webhook", true, ErrInvalidScheme},
		{"null byte", "http://example.com%00.evil.com/", true, ErrInvalidURL}, // url.Parse fails
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWebhookURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWebhookURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errType != nil && err != tt.errType {
				t.Errorf("ValidateWebhookURL(%q) error = %v, want %v", tt.url, err, tt.errType)
			}
		})
	}
}

func TestValidateIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{"public IP", "8.8.8.8", false},
		{"public IP 2", "1.1.1.1", false},
		{"loopback", "127.0.0.1", true},
		{"loopback IPv6", "::1", true},
		{"private 10.x", "10.0.0.1", true},
		{"private 172.16.x", "172.16.0.1", true},
		{"private 192.168.x", "192.168.0.1", true},
		{"link-local", "169.254.1.1", true},
		{"AWS metadata", "169.254.169.254", true},
		{"unspecified", "0.0.0.0", true},
		{"Alibaba metadata", "100.100.100.200", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := parseIP(tt.ip)
			if ip == nil {
				t.Fatalf("invalid test IP: %s", tt.ip)
			}
			err := validateIP(ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIP(%s) error = %v, wantErr %v", tt.ip, err, tt.wantErr)
			}
		})
	}
}

func TestContainsSuspiciousPatterns(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://example.com/webhook", false},
		{"http://0x7f000001/", true},
		{"http://2130706433/", true},
		{"http://evil@127.0.0.1/", true},
		{"http://127.0.0.1#@example.com/", true},
		{"http://example.com%00.evil.com/", true},
		{"http://[::1]/", true},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := containsSuspiciousPatterns(tt.url)
			if got != tt.want {
				t.Errorf("containsSuspiciousPatterns(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestIsLocalhostVariation(t *testing.T) {
	tests := []struct {
		hostname string
		want     bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"127.1", true},
		{"0.0.0.0", true},
		{"example.localhost", true},
		{"127.0.0.1.nip.io", true},
		{"test.xip.io", true},
		{"localtest.me", false}, // Not in isLocalhostVariation, but in blockedHosts
		{"example.com", false},
		{"api.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			got := isLocalhostVariation(tt.hostname)
			if got != tt.want {
				t.Errorf("isLocalhostVariation(%q) = %v, want %v", tt.hostname, got, tt.want)
			}
		})
	}
}

func parseIP(s string) net.IP {
	return net.ParseIP(s)
}
