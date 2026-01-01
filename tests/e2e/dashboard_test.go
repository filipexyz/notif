package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

// TestDashboardAuthRequired verifies API key management routes require Clerk authentication.
// Note: Full Clerk JWT testing requires integration with Clerk's test environment.
// These tests verify that routes properly reject unauthenticated requests.
func TestDashboardAuthRequired(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	tests := []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
	}{
		{
			name:       "create api key requires auth",
			method:     "POST",
			path:       "/api/v1/api-keys",
			body:       map[string]string{"name": "test"},
			wantStatus: http.StatusUnauthorized, // No auth provided
		},
		{
			name:       "list api keys requires auth",
			method:     "GET",
			path:       "/api/v1/api-keys",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "revoke api key requires auth",
			method:     "DELETE",
			path:       "/api/v1/api-keys/00000000-0000-0000-0000-000000000000",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			var err error

			if tt.body != nil {
				bodyBytes, _ := json.Marshal(tt.body)
				req, err = http.NewRequest(tt.method, env.ServerURL+tt.path, bytes.NewReader(bodyBytes))
			} else {
				req, err = http.NewRequest(tt.method, env.ServerURL+tt.path, nil)
			}
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

// TestAPIKeyManagementRejectsAPIKeyAuth verifies that API key management routes
// don't accept API key authentication (they require Clerk JWT).
func TestAPIKeyManagementRejectsAPIKeyAuth(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"create with api key", "POST", "/api/v1/api-keys"},
		{"list with api key", "GET", "/api/v1/api-keys"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.method == "POST" {
				body, _ = json.Marshal(map[string]string{"name": "test"})
			}

			req, err := http.NewRequest(tt.method, env.ServerURL+tt.path, bytes.NewReader(body))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			// Try using API key auth (should be rejected - requires Clerk auth)
			req.Header.Set("Authorization", "Bearer "+TestAPIKey)
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			// API key management should reject API key auth (returns 403)
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("got status %d, want %d (api-keys should reject API key auth)", resp.StatusCode, http.StatusForbidden)
			}
		})
	}
}
