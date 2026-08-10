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
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/audit"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/integration"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
	"github.com/go-chi/chi/v5"
)

// Integration connectors.
//
// Every handler here resolves the tenant from the session and passes it down.
// The registry used to be a process-global map with no tenant column, so one
// tenant's administrator listed — and dispatched to — every other tenant's
// connectors.

// integrationSaveRequest is the wire shape of the connector form.
type integrationSaveRequest struct {
	Provider  string            `json:"provider"`
	Name      string            `json:"name"`
	TargetURL string            `json:"target_url"`
	Secret    string            `json:"secret_key"`
	Status    string            `json:"status"`
	Config    map[string]string `json:"config"`
}

func (r integrationSaveRequest) toSave() integration.SaveRequest {
	return integration.SaveRequest{
		Provider:  integration.Provider(r.Provider),
		Name:      r.Name,
		TargetURL: r.TargetURL,
		Secret:    r.Secret,
		Status:    integration.ConnectorStatus(r.Status),
		Config:    r.Config,
	}
}

// integrationError maps a manager error onto a status.
//
// A connector that belongs to another tenant is reported as missing, not as
// forbidden: the distinction would confirm the id exists. Everything the
// administrator can act on carries its own sentence; everything else is a 500
// with the detail in the log rather than in the browser, which is where a
// driver's message about a constraint belongs.
func integrationError(w http.ResponseWriter, err error) {
	var invalid *integration.InvalidError
	switch {
	case errors.Is(err, integration.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "integration not found")
	case errors.Is(err, integration.ErrDuplicateName):
		httpx.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, integration.ErrNoEncryptionKey),
		errors.Is(err, integration.ErrProviderUnavailable):
		httpx.Error(w, http.StatusServiceUnavailable, err.Error())
	case errors.As(err, &invalid):
		httpx.Error(w, http.StatusBadRequest, invalid.Error())
	default:
		slog.Error("integration: request failed", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "the integration could not be saved")
	}
}

func (s *Server) handleListIntegrations(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	list, err := s.integrationMgr.List(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load integrations")
		return
	}
	httpx.JSON(w, http.StatusOK, list)
}

// handleIntegrationProviders tells the screen which connectors this deployment
// can actually offer, so an administrator is not given a form for a provider
// whose OAuth client was never configured.
func (s *Server) handleIntegrationProviders(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{
		"providers":             integration.Catalog(),
		"encryption_configured": integration.EncryptionConfigured(),
		"redirect_uri":          integration.RedirectURI(),
	})
}

func (s *Server) handleRegisterIntegration(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	var req integrationSaveRequest
	if decodeLimitedJSON(r, &req, 32<<10) != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid integration configuration")
		return
	}
	conn, err := s.integrationMgr.Create(r.Context(), tenantID, req.toSave())
	if err != nil {
		integrationError(w, err)
		return
	}
	claims, _ := auth.UserFromContext(r.Context())
	audit.Record(r.Context(), tenantID, claims.UserID, "integration.create", "integration",
		map[string]any{"id": conn.ID, "provider": conn.Provider})
	httpx.JSON(w, http.StatusCreated, conn)
}

func (s *Server) handleUpdateIntegration(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	var req integrationSaveRequest
	if decodeLimitedJSON(r, &req, 32<<10) != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid integration configuration")
		return
	}
	conn, err := s.integrationMgr.Update(r.Context(), tenantID, chi.URLParam(r, "id"), req.toSave())
	if err != nil {
		integrationError(w, err)
		return
	}
	claims, _ := auth.UserFromContext(r.Context())
	audit.Record(r.Context(), tenantID, claims.UserID, "integration.update", "integration",
		map[string]any{"id": conn.ID})
	httpx.JSON(w, http.StatusOK, conn)
}

func (s *Server) handleDeleteIntegration(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.integrationMgr.Delete(r.Context(), tenantID, id); err != nil {
		integrationError(w, err)
		return
	}
	claims, _ := auth.UserFromContext(r.Context())
	audit.Record(r.Context(), tenantID, claims.UserID, "integration.delete", "integration",
		map[string]any{"id": id})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleConnectIntegration starts the OAuth grant and answers with the URL to
// send the administrator to. It returns the URL rather than redirecting so the
// caller is an ordinary fetch from the settings screen.
func (s *Server) handleConnectIntegration(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	claims, _ := auth.UserFromContext(r.Context())
	authURL, err := s.integrationMgr.BeginConnect(r.Context(), tenantID, claims.UserID, chi.URLParam(r, "id"))
	if err != nil {
		integrationError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"authorization_url": authURL})
}

func (s *Server) handleDisconnectIntegration(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.integrationMgr.Disconnect(r.Context(), tenantID, id); err != nil {
		integrationError(w, err)
		return
	}
	claims, _ := auth.UserFromContext(r.Context())
	audit.Record(r.Context(), tenantID, claims.UserID, "integration.disconnect", "integration",
		map[string]any{"id": id})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}

// handleIntegrationDeliveries returns what has recently left the platform. A
// signed document reaching an outside account is a disclosure, and the record
// of it is the only thing that can answer for it afterwards.
func (s *Server) handleIntegrationDeliveries(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, convErr := strconv.Atoi(raw); convErr == nil {
			limit = parsed
		}
	}
	list, err := s.integrationMgr.Deliveries(r.Context(), tenantID, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load delivery history")
		return
	}
	httpx.JSON(w, http.StatusOK, list)
}

// handleIntegrationOAuthCallback receives the provider's redirect.
//
// It is deliberately outside the authenticated API group: the browser arrives
// here from Google or Dropbox, and whether it still carries a session cookie
// depends on SameSite and on which tab the consent screen opened in. Authority
// comes from the single-use state row instead, which binds the callback to the
// tenant, the connector and the administrator who started it.
func (s *Server) handleIntegrationOAuthCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	settingsURL := strings.TrimRight(os.Getenv("PUBLIC_ORIGIN"), "/") + "/settings/integrations"
	if strings.TrimSpace(settingsURL) == "/settings/integrations" {
		settingsURL = "/settings/integrations"
	}

	if providerErr := query.Get("error"); providerErr != "" {
		// The citizen-facing half of this is an administrator who pressed
		// "Cancel" on a consent screen; that is not a server error.
		http.Redirect(w, r, settingsURL+"?connected=0&reason="+url.QueryEscape(providerErr), http.StatusFound)
		return
	}

	res, err := s.integrationMgr.CompleteConnect(r.Context(), query.Get("state"), query.Get("code"))
	if err != nil {
		slog.Warn("integration: oauth callback failed", "error", err)
		http.Redirect(w, r, settingsURL+"?connected=0&reason="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}

	// Binding an outside account to a tenant is the act this whole flow exists
	// to perform, and it was the one integration action that left no record.
	// The actor is the administrator who began the flow, carried here by the
	// state row — there is no session on this request to read one from.
	audit.Record(r.Context(), res.TenantID, res.UserID, "integration.connected", "integration",
		map[string]any{
			"id":            res.Connector.ID,
			"provider":      res.Connector.Provider,
			"account_label": res.Connector.AccountLabel,
		})

	http.Redirect(w, r, settingsURL+"?connected=1&name="+url.QueryEscape(res.Connector.Name), http.StatusFound)
}
