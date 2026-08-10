/*
 * Gerege SSO
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Package platform provides the core HTTP Server orchestrator, routing table,
 * authentication middleware, and app installer wiring.
 */

package platform

import (
	"errors"
	"net/http"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/memo"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/rbac"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
	"github.com/jackc/pgx/v5"
)

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := auth.TokenFromRequest(r)
		if token == "" {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized: missing session token")
			return
		}

		claims, err := s.sessions.Resolve(r.Context(), token)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized: invalid or expired session")
			return
		}

		ctx := auth.WithUserContext(r.Context(), claims)
		ctx = tenant.WithTenantID(ctx, claims.TenantID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireAdmin gates tenant-administrative endpoints. It must be layered after
// authMiddleware.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.UserFromContext(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !claims.IsAdmin {
			httpx.Error(w, http.StatusForbidden, "forbidden: tenant administrator role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) appGateMiddleware(appID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, ok := tenant.Require(w, r)
			if !ok {
				return
			}

			// Whether a tenant has this app is asked on the way into every
			// request the app serves, and answered from a row that changes when
			// an administrator presses Install — a few times in the life of a
			// deployment. The negative answer is cached too: a client polling an
			// app the tenant does not have should not cost a query each time.
			cacheKey := memo.Key(tenantID, appID)
			enabled, cached := s.appGate.Get(cacheKey)
			if !cached {
				err := s.db.QueryRow(r.Context(),
					`SELECT enabled FROM app_installations WHERE tenant_id = $1 AND app_id = $2`,
					tenantID, appID).Scan(&enabled)
				// Only a definite answer is kept. A database that is down would
				// otherwise pin "not installed" onto the tenant for the length of
				// the entry, and the app would stay missing after it came back.
				if err == nil || errors.Is(err, pgx.ErrNoRows) {
					s.appGate.Put(cacheKey, err == nil && enabled)
				}
			}

			if !enabled {
				httpx.Error(w, http.StatusForbidden, "forbidden: app module "+appID+" is not installed or enabled for this tenant")
				return
			}

			// Model-level access rights are additive across all assigned roles,
			// matching Odoo's ir.model.access behaviour. Government workflow has
			// its own action- and unit-aware permission checks.
			if permission := appRequestPermission(appID, r.Method, r.URL.Path); permission != "" {
				rbac.RequirePermission(s.permissions, permission)(next).ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

func appRequestPermission(appID, method, path string) string {
	prefixes := map[string]string{
		"io.example.contacts": "contacts", "io.example.products": "products",
		"io.example.inventory": "inventory", "io.example.billing": "billing",
		"io.example.developer_portal": "developer",
	}
	prefix := prefixes[appID]
	if prefix == "" {
		return ""
	}
	if method == http.MethodGet || method == http.MethodHead {
		return prefix + ".read"
	}
	return prefix + ".manage"
}
