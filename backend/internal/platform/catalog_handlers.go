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
	"encoding/json"
	"net/http"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/appcatalog"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/config"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/menu"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/security"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleMenus(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	menus, err := menu.GetTenantMenus(r.Context(), s.installer, tenantID, config.LocaleFromRequest(r))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to fetch menus")
		return
	}
	claims, _ := auth.UserFromContext(r.Context())
	if !claims.IsAdmin {
		permissions, permissionErr := s.permissions.GetUserPermissions(r.Context(), tenantID, claims.UserID)
		if permissionErr != nil {
			httpx.Error(w, http.StatusInternalServerError, "failed to resolve menu access")
			return
		}
		visible := menus[:0]
		for _, item := range menus {
			permission := appReadPermission(item.AppID)
			if permission == "" || permissions[permission] {
				visible = append(visible, item)
			}
		}
		menus = visible
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(menus)
}

func appReadPermission(appID string) string {
	switch appID {
	case "io.example.contacts":
		return "contacts.read"
	case "io.example.products":
		return "products.read"
	case "io.example.inventory":
		return "inventory.read"
	case "io.example.billing":
		return "billing.read"
	case "io.example.documents":
		return "documents.read"
	case "io.example.developer_portal":
		return "developer.read"
	case "io.example.gov_services":
		return "gov.read"
	default:
		return ""
	}
}

func (s *Server) handleListStoreApps(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := tenant.FromContext(r.Context())
	catalog := s.installer.GetCatalog()

	// "installed" and "enabled" are distinct states: an app can be installed
	// and then disabled. Deriving both from the enabled-only query reported
	// disabled apps as never installed, so the UI offered "Install" again.
	installedStates, err := s.installer.GetInstallationStatesForTenant(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load installed apps")
		return
	}

	type StoreAppResponse struct {
		appcatalog.CatalogApp
		Installed bool `json:"installed"`
		Enabled   bool `json:"enabled"`
	}

	locale := config.LocaleFromRequest(r)
	res := make([]StoreAppResponse, 0, len(catalog))
	for _, app := range catalog {
		state, installed := installedStates[app.ID]
		res = append(res, StoreAppResponse{
			CatalogApp: app.Localized(locale),
			Installed:  installed,
			Enabled:    state,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleGetStoreApp(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		httpx.Error(w, http.StatusBadRequest, "invalid app slug format")
		return
	}

	app, ok := s.installer.GetAppBySlug(slug)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "app not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(app.Localized(config.LocaleFromRequest(r)))
}

func (s *Server) handleListInstalledApps(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	rows, err := s.db.Query(r.Context(),
		`SELECT ai.id, ai.app_id, a.slug, a.name, ai.installed_version, ai.status, ai.enabled, ai.installed_at
		 FROM app_installations ai
		 JOIN apps a ON a.id = ai.app_id
		 WHERE ai.tenant_id = $1`, tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type InstalledApp struct {
		ID               string    `json:"id"`
		AppID            string    `json:"app_id"`
		Slug             string    `json:"slug"`
		Name             string    `json:"name"`
		InstalledVersion string    `json:"installed_version"`
		Status           string    `json:"status"`
		Enabled          bool      `json:"enabled"`
		InstalledAt      time.Time `json:"installed_at"`
	}

	locale := config.LocaleFromRequest(r)
	list := make([]InstalledApp, 0)
	for rows.Next() {
		var item InstalledApp
		// Skipping unreadable rows reported a tenant's app as not installed,
		// and the store then offered to install it again over the top.
		if err := rows.Scan(&item.ID, &item.AppID, &item.Slug, &item.Name, &item.InstalledVersion, &item.Status, &item.Enabled, &item.InstalledAt); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "failed to read installed apps")
			return
		}
		// apps.name is the manifest's English name, and this was the one catalogue
		// surface that answered with it: the store and the sidebar both resolve
		// through the caller's locale. So an installed app was called "Digital
		// Documents & Signatures" here and "Баримт ба цахим гарын үсэг" in the menu
		// beside it — the same app under two names, which reads as an app that is
		// installed and yet missing from the menu.
		if catalogApp, ok := s.installer.GetAppBySlug(item.Slug); ok {
			if localized := catalogApp.Localized(locale).Name; localized != "" {
				item.Name = localized
			}
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to read installed apps")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (s *Server) handleInstallApp(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		httpx.Error(w, http.StatusBadRequest, "invalid app slug format")
		return
	}

	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := s.installer.InstallApp(r.Context(), claims.TenantID, slug, claims.UserID); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	// The app gate reads a cached copy of this row, so the screen that just
	// pressed the button has to stop being told the old answer.
	s.forgetAppGate(claims.TenantID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "installed", "app": slug})
}

func (s *Server) handleDisableApp(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		httpx.Error(w, http.StatusBadRequest, "invalid app slug format")
		return
	}

	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := s.installer.DisableApp(r.Context(), claims.TenantID, slug, claims.UserID); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	// The app gate reads a cached copy of this row, so the screen that just
	// pressed the button has to stop being told the old answer.
	s.forgetAppGate(claims.TenantID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "disabled", "app": slug})
}

func (s *Server) handleEnableApp(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		httpx.Error(w, http.StatusBadRequest, "invalid app slug format")
		return
	}

	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := s.installer.EnableApp(r.Context(), claims.TenantID, slug, claims.UserID); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	// The app gate reads a cached copy of this row, so the screen that just
	// pressed the button has to stop being told the old answer.
	s.forgetAppGate(claims.TenantID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "enabled", "app": slug})
}
