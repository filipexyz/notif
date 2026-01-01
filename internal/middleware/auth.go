package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/domain"
)

type contextKey string

const apiKeyContextKey contextKey = "apiKey"

// Auth is the authentication middleware.
type Auth struct {
	queries *db.Queries
}

// NewAuth creates a new Auth middleware.
func NewAuth(queries *db.Queries) *Auth {
	return &Auth{queries: queries}
}

// Handler returns the middleware handler.
func (a *Auth) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		// Validate format
		if !domain.ValidateKeyFormat(token) {
			writeError(w, http.StatusUnauthorized, "invalid api key format")
			return
		}

		// Hash and lookup
		keyHash := hashKey(token)
		apiKey, err := a.queries.GetAPIKeyByHash(r.Context(), keyHash)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid api key")
			return
		}

		// Update last used (async, don't block request)
		go func() {
			_ = a.queries.UpdateAPIKeyLastUsed(context.Background(), apiKey.ID)
		}()

		// Add to context
		ctx := context.WithValue(r.Context(), apiKeyContextKey, &apiKey)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetAPIKey retrieves the API key from the request context.
func GetAPIKey(ctx context.Context) *db.GetAPIKeyByHashRow {
	key, _ := ctx.Value(apiKeyContextKey).(*db.GetAPIKeyByHashRow)
	return key
}

// GetAPIKeyOrgID retrieves the organization ID from the API key in context.
// Returns empty string if no API key or no org_id is set.
func GetAPIKeyOrgID(ctx context.Context) string {
	key := GetAPIKey(ctx)
	if key == nil || !key.OrgID.Valid {
		return ""
	}
	return key.OrgID.String
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		// Also check query param for WebSocket
		return r.URL.Query().Get("token")
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}

	return parts[1]
}

// ClerkQueryParamAuth moves query param 'token' to Authorization header
// for WebSocket connections so Clerk middleware can process it.
func ClerkQueryParamAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no Authorization header but token query param exists, move it
			if r.Header.Get("Authorization") == "" {
				if token := r.URL.Query().Get("token"); token != "" {
					r.Header.Set("Authorization", "Bearer "+token)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
