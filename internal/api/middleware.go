package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"golang-nextjs/internal/auth"
	"golang-nextjs/internal/db"
)

type ctxKey string

const (
	ctxTenantID ctxKey = "tenant_id"
	ctxUserID   ctxKey = "user_id"
	ctxRole     ctxKey = "role"
)

// RequireAuth authenticates a request by its bearer token: the
// token's SHA-256 hash is looked up against users.token_hash, and the
// matching user's tenant ID, user ID, and role are injected into
// context. Each caller is identified individually — unlike the earlier
// single-shared-token design, where every caller was authenticated as
// the same fixed dev user regardless of who actually held the token
// (NFR-4, NFR-5).
func RequireAuth(users *db.UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid bearer token")
				return
			}

			user, err := users.GetByTokenHash(r.Context(), auth.HashToken(token))
			if errors.Is(err, db.ErrUserNotFound) {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid bearer token")
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to authenticate")
				return
			}

			ctx := context.WithValue(r.Context(), ctxTenantID, user.TenantID)
			ctx = context.WithValue(ctx, ctxUserID, user.ID)
			ctx = context.WithValue(ctx, ctxRole, user.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole rejects the request unless the authenticated user's role
// (set into context by RequireAuth, which must run first) is one of
// allowedRoles (SRS Feature: Human Review — "only authorized reviewers
// can act on a review task").
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !slices.Contains(allowedRoles, roleFromContext(r.Context())) {
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

func roleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxRole).(string)
	return v
}
