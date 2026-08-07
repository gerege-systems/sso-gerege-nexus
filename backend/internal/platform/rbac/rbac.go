package rbac

import (
	"context"
	"errors"
	"net/http"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
)

type PermissionStore interface {
	GetUserPermissions(ctx context.Context, tenantID, userID string) (map[string]bool, error)
}

var ErrForbidden = errors.New("forbidden: insufficient permissions")

// RequirePermission returns an HTTP middleware that enforces server-side permission authorization.
func RequirePermission(store PermissionStore, permissionCode string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := auth.UserFromContext(r.Context())
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// Admin user bypasses individual permission checks
			if claims.IsAdmin {
				next.ServeHTTP(w, r)
				return
			}

			perms, err := store.GetUserPermissions(r.Context(), claims.TenantID, claims.UserID)
			if err != nil || !perms[permissionCode] {
				http.Error(w, `{"error":"forbidden: permission `+permissionCode+` required"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
