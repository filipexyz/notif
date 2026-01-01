package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

// TestDashboardAuthRequired verifies dashboard routes require Clerk authentication.
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
			path:       "/dashboard/api-keys",
			body:       map[string]string{"name": "test", "environment": "test"},
			wantStatus: http.StatusForbidden, // Clerk returns 403 for missing/invalid auth
		},
		{
			name:       "list api keys requires auth",
			method:     "GET",
			path:       "/dashboard/api-keys",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "revoke api key requires auth",
			method:     "DELETE",
			path:       "/dashboard/api-keys/00000000-0000-0000-0000-000000000000",
			wantStatus: http.StatusForbidden,
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

// TestDashboardRejectsAPIKeyAuth verifies that dashboard routes don't accept
// API key authentication (they require Clerk JWT).
func TestDashboardRejectsAPIKeyAuth(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"create with api key", "POST", "/dashboard/api-keys"},
		{"list with api key", "GET", "/dashboard/api-keys"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.method == "POST" {
				body, _ = json.Marshal(map[string]string{"name": "test", "environment": "test"})
			}

			req, err := http.NewRequest(tt.method, env.ServerURL+tt.path, bytes.NewReader(body))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			// Try using API key auth (should be rejected)
			req.Header.Set("Authorization", "Bearer "+TestAPIKey)
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			// Dashboard should reject API key auth (Clerk returns 403)
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("got status %d, want %d (dashboard should reject API key auth)", resp.StatusCode, http.StatusForbidden)
			}
		})
	}
}
