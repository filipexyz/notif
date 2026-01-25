package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/filipexyz/notif/internal/config"
	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/domain"
	"github.com/google/uuid"
)

type authContextKey string

const authCtxKey authContextKey = "authContext"

// AuthContext holds the authenticated user/key context.
// Either APIKeyID or UserID will be set, not both.
type AuthContext struct {
	OrgID     string
	ProjectID string     // Project ID - derived from API key or X-Project-ID header
	APIKeyID  *uuid.UUID // Set if authenticated via API key
	UserID    *string    // Set if authenticated via Clerk
}

// UnifiedAuth creates middleware that accepts both API key and Clerk auth.
// API key takes precedence if both are present.
// In self-hosted mode (AUTH_MODE=none), Clerk auth is skipped.
func UnifiedAuth(queries *db.Queries, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var authCtx *AuthContext

			// 1. Try API key first (Bearer nsh_xxx)
			token := extractBearerToken(r)
			if token != "" && strings.HasPrefix(token, "nsh_") {
				if domain.ValidateKeyFormat(token) {
					keyHash := hashKey(token)
					apiKey, err := queries.GetAPIKeyByHash(r.Context(), keyHash)
					if err == nil {
						// Valid API key - derive project from API key
						keyID := uuid.UUID(apiKey.ID.Bytes)
						authCtx = &AuthContext{
							OrgID:     apiKey.OrgID.String,
							ProjectID: apiKey.ProjectID,
							APIKeyID:  &keyID,
						}

						// Update last used (async)
						go func() {
							_ = queries.UpdateAPIKeyLastUsed(context.Background(), apiKey.ID)
						}()

						// Also store API key in context for handlers that need it
						ctx := context.WithValue(r.Context(), apiKeyContextKey, &apiKey)
						ctx = context.WithValue(ctx, authCtxKey, authCtx)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
				// Invalid API key format or key not found
				writeError(w, http.StatusUnauthorized, "invalid api key")
				return
			}

			// 2. In self-hosted mode, skip Clerk entirely
			if cfg.IsSelfHosted() {
				// No API key provided in self-hosted mode
				writeError(w, http.StatusUnauthorized, "api key required")
				return
			}

			// 3. Try Clerk session (only in clerk mode)
			claims, ok := clerk.SessionClaimsFromContext(r.Context())
			if ok && claims.Subject != "" {
				userID := claims.Subject
				// Use org ID if available, otherwise fall back to user ID for personal accounts
				orgID := claims.ActiveOrganizationID
				if orgID == "" {
					orgID = claims.Subject // personal account uses user_xxx as org
				}

				// Get project from X-Project-ID header, query param, or use default
				// Query param is used for WebSocket connections
				projectID := r.Header.Get("X-Project-ID")
				if projectID == "" {
					projectID = r.URL.Query().Get("project_id")
				}
				if projectID == "" {
					// Get or create default project for org
					project, err := queries.GetOrCreateDefaultProject(r.Context(), db.GetOrCreateDefaultProjectParams{
						ID:    domain.GenerateProjectID(),
						OrgID: orgID,
					})
					if err != nil {
						writeError(w, http.StatusInternalServerError, "failed to get default project")
						return
					}
					projectID = project.ID
				}

				authCtx = &AuthContext{
					OrgID:     orgID,
					ProjectID: projectID,
					UserID:    &userID,
				}

				// Store clerk session for handlers that need it
				session := &ClerkSession{
					UserID: claims.Subject,
					OrgID:  orgID,
				}
				ctx := context.WithValue(r.Context(), clerkSessionKey, session)
				ctx = context.WithValue(ctx, authCtxKey, authCtx)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// 4. No valid auth
			writeError(w, http.StatusUnauthorized, "unauthorized")
		})
	}
}

// RequireClerkAuth returns middleware that requires Clerk auth (not API key).
// Use this for endpoints like API key management that shouldn't allow API key auth.
// In self-hosted mode (AUTH_MODE=none), this allows API key auth instead.
func RequireClerkAuth(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCtx := GetAuthContext(r.Context())
			if authCtx == nil {
				writeError(w, http.StatusForbidden, "authentication required")
				return
			}

			// In self-hosted mode, allow API key auth for dashboard routes
			if cfg.IsSelfHosted() {
				if authCtx.APIKeyID != nil {
					next.ServeHTTP(w, r)
					return
				}
				writeError(w, http.StatusForbidden, "api key required")
				return
			}

			// In clerk mode, require actual Clerk auth
			if authCtx.UserID == nil {
				writeError(w, http.StatusForbidden, "clerk authentication required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetAuthContext retrieves the auth context from the request.
func GetAuthContext(ctx context.Context) *AuthContext {
	authCtx, _ := ctx.Value(authCtxKey).(*AuthContext)
	return authCtx
}

// GetOrgIDFromContext retrieves the org ID from auth context.
func GetOrgIDFromContext(ctx context.Context) string {
	authCtx := GetAuthContext(ctx)
	if authCtx == nil {
		return ""
	}
	return authCtx.OrgID
}

// GetProjectIDFromContext retrieves the project ID from auth context.
func GetProjectIDFromContext(ctx context.Context) string {
	authCtx := GetAuthContext(ctx)
	if authCtx == nil {
		return ""
	}
	return authCtx.ProjectID
}
