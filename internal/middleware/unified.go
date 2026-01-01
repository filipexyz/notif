package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/domain"
	"github.com/google/uuid"
)

type authContextKey string

const authCtxKey authContextKey = "authContext"

// AuthContext holds the authenticated user/key context.
// Either APIKeyID or UserID will be set, not both.
type AuthContext struct {
	OrgID    string
	APIKeyID *uuid.UUID // Set if authenticated via API key
	UserID   *string    // Set if authenticated via Clerk
}

// UnifiedAuth creates middleware that accepts both API key and Clerk auth.
// API key takes precedence if both are present.
func UnifiedAuth(queries *db.Queries) func(http.Handler) http.Handler {
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
						// Valid API key
						keyID := uuid.UUID(apiKey.ID.Bytes)
						authCtx = &AuthContext{
							OrgID:    apiKey.OrgID.String,
							APIKeyID: &keyID,
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

			// 2. Try Clerk session
			claims, ok := clerk.SessionClaimsFromContext(r.Context())
			if ok && claims.Subject != "" {
				userID := claims.Subject
				// Use org ID if available, otherwise fall back to user ID for personal accounts
				orgID := claims.ActiveOrganizationID
				if orgID == "" {
					orgID = claims.Subject // personal account uses user_xxx as org
				}

				authCtx = &AuthContext{
					OrgID:  orgID,
					UserID: &userID,
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

			// 3. No valid auth
			writeError(w, http.StatusUnauthorized, "unauthorized")
		})
	}
}

// RequireClerkAuth returns middleware that requires Clerk auth (not API key).
// Use this for endpoints like API key management that shouldn't allow API key auth.
func RequireClerkAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCtx := GetAuthContext(r.Context())
			if authCtx == nil || authCtx.UserID == nil {
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
