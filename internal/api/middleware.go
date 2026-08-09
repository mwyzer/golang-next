package api

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"golang-nextjs/internal/db"
)

type ctxKey string

const (
	ctxTenantID ctxKey = "tenant_id"
	ctxUserID   ctxKey = "user_id"

	// devTenantID/devUserID match the seeded row in
	// db/migrations/000002_seed_dev_data.up.sql. Real per-user
	// authentication is out of scope for this slice (see PRD Open
	// Questions); this shared-token auth stands in for NFR-4/NFR-5
	// until it's built.
	devTenantID = "00000000-0000-0000-0000-000000000001"
	devUserID   = "00000000-0000-0000-0000-000000000002"
)

// RequireAuth checks a shared bearer token and injects the (currently
// fixed, dev-seeded) tenant and user IDs into the request context.
func RequireAuth(apiToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" || token != apiToken {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid bearer token")
				return
			}

			ctx := context.WithValue(r.Context(), ctxTenantID, devTenantID)
			ctx = context.WithValue(ctx, ctxUserID, devUserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole rejects the request unless the authenticated user's role
// (from the users table) is one of allowedRoles. Must run after
// RequireAuth so a user ID is already in context (SRS Feature: Human
// Review — "only authorized reviewers can act on a review task").
func RequireRole(users *db.UserRepo, allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, err := users.GetRole(r.Context(), userIDFromContext(r.Context()))
			if err != nil {
				writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to load user role")
				return
			}
			if !slices.Contains(allowedRoles, role) {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "user is not authorized for this action")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func tenantIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxTenantID).(string)
	return v
}

func userIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserID).(string)
	return v
}
