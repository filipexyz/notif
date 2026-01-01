package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
)

type clerkContextKey string

const (
	clerkSessionKey clerkContextKey = "clerkSession"
)

// ClerkSession holds extracted session information from Clerk JWT.
type ClerkSession struct {
	UserID string
	OrgID  string
}

// ClerkAuth is the Clerk JWT authentication middleware for dashboard routes.
type ClerkAuth struct{}

// NewClerkAuth creates a new ClerkAuth middleware.
func NewClerkAuth() *ClerkAuth {
	return &ClerkAuth{}
}

// Handler returns the middleware handler for Clerk JWT validation.
// Requires CLERK_SECRET_KEY to be set via clerk.SetKey() before use.
func (c *ClerkAuth) Handler(next http.Handler) http.Handler {
	// Use Clerk's built-in middleware for JWT validation
	return clerkhttp.RequireHeaderAuthorization()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := clerk.SessionClaimsFromContext(r.Context())
			if !ok {
				writeClerkError(w, http.StatusUnauthorized, "invalid or missing session")
				return
			}

			// Extract org_id from claims (Clerk includes this for active org)
			orgID := ""
			if claims.ActiveOrganizationID != "" {
				orgID = claims.ActiveOrganizationID
			}

			// Require organization context for all dashboard operations
			if orgID == "" {
				writeClerkError(w, http.StatusForbidden, "organization context required - please select an organization")
				return
			}

			session := &ClerkSession{
				UserID: claims.Subject,
				OrgID:  orgID,
			}

			ctx := context.WithValue(r.Context(), clerkSessionKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		}),
	)
}

// GetClerkSession retrieves the Clerk session from context.
func GetClerkSession(ctx context.Context) *ClerkSession {
	session, _ := ctx.Value(clerkSessionKey).(*ClerkSession)
	return session
}

// GetOrgID retrieves the organization ID from Clerk session context.
func GetOrgID(ctx context.Context) string {
	session := GetClerkSession(ctx)
	if session == nil {
		return ""
	}
	return session.OrgID
}

func writeClerkError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
