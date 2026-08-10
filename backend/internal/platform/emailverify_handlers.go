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
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/audit"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/config"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/emailverify"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/security"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
)

// Email verification.
//
// The mail is sent by the hosted service; this platform asks for it and finds
// out when the link was followed. Two endpoints, and only one of them is for
// a person: /verify/send is called by a signed-in screen (app modules call the
// Go API directly), and /verify/landed is where the recipient's browser arrives
// after the service has honoured its own token.

// emailVerifyError maps a service error onto a status.
//
// The distinction that matters to an integrator is retryable versus not: a rate
// limit and an upstream failure will work later, a malformed address will not,
// and a missing key is nobody's fault but the operator's.
func emailVerifyError(w http.ResponseWriter, err error) {
	var invalid *emailverify.InvalidError
	var limited *emailverify.RateLimitedError
	switch {
	case errors.As(err, &invalid):
		httpx.Error(w, http.StatusBadRequest, invalid.Error())
	case errors.As(err, &limited):
		w.Header().Set("Retry-After", strconv.Itoa(int(limited.RetryAfter.Round(time.Second).Seconds())))
		httpx.Error(w, http.StatusTooManyRequests, limited.Error())
	case errors.Is(err, emailverify.ErrNotConfigured),
		errors.Is(err, emailverify.ErrOriginNotHTTPS),
		errors.Is(err, emailverify.ErrUnauthorizedKey):
		// All three are this deployment's configuration, not the request's.
		// 503 says "not right now, and not because of what you sent".
		httpx.Error(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, emailverify.ErrUpstream):
		httpx.Error(w, http.StatusBadGateway, err.Error())
	case errors.Is(err, emailverify.ErrLinkSpent):
		httpx.Error(w, http.StatusGone, err.Error())
	default:
		slog.Error("emailverify: request failed", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "the verification request could not be completed")
	}
}

type verifySendRequest struct {
	Email       string `json:"email"`
	RedirectURL string `json:"redirect_url"`
	Purpose     string `json:"purpose"`
}

// handleVerifySend asks the hosted service for a link.
//
// It is inside the authenticated group: the caller is one of this platform's
// own screens. A system outside Gerege SSO does not come through here at all
// — it holds its own key with the verification service and calls that service
// directly, which is why this platform no longer issues keys.
func (s *Server) handleVerifySend(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	var req verifySendRequest
	if decodeLimitedJSON(r, &req, 8<<10) != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid verification request")
		return
	}

	claims, _ := auth.UserFromContext(r.Context())
	verification, err := s.emailVerify.Send(r.Context(), tenantID, emailverify.Request{
		Email:       req.Email,
		RedirectURL: req.RedirectURL,
		Purpose:     req.Purpose,
		Source:      "portal",
		ClientIP:    security.ClientIP(r),
	})
	if err != nil {
		emailVerifyError(w, err)
		return
	}
	audit.Record(r.Context(), tenantID, claims.UserID, "emailverify.send", "email_verification",
		map[string]any{"id": verification.ID, "purpose": verification.Purpose})
	httpx.JSON(w, http.StatusOK, verification)
}

// handleVerifyLanded receives the person the verification service sends back.
//
// Unauthenticated by design: whoever is arriving has just proved an address,
// not signed in, and may have no account here at all. The single-use reference
// in the query is the whole authority, and it is claimed exactly once — the
// return address travels through a mailbox and then a browser's history, so a
// replay of it must not be able to re-assert anything.
func (s *Server) handleVerifyLanded(w http.ResponseWriter, r *http.Request) {
	locale := config.LocaleFromRequest(r)
	verification, err := s.emailVerify.Confirm(r.Context(), r.URL.Query().Get("ref"))
	if err != nil {
		if !errors.Is(err, emailverify.ErrLinkSpent) {
			slog.Error("emailverify: landing failed", "error", err)
			writeVerifyPage(w, http.StatusInternalServerError, locale, false)
			return
		}
		writeVerifyPage(w, http.StatusGone, locale, false)
		return
	}

	audit.Record(r.Context(), verification.TenantID, "recipient", "emailverify.confirmed", "email_verification",
		map[string]any{"id": verification.ID, "source": verification.Source, "purpose": verification.Purpose})

	// The destination was validated when the request was made, against the
	// rules in force then. It is not re-derived from anything in this request.
	if verification.RedirectURL != "" {
		http.Redirect(w, r, verification.RedirectURL, http.StatusFound)
		return
	}
	writeVerifyPage(w, http.StatusOK, locale, true)
}

// writeVerifyPage answers a click with a page rather than a JSON body: what
// arrives here is a person in a browser who never asked for an API.
func writeVerifyPage(w http.ResponseWriter, status int, locale string, confirmed bool) {
	title, body := emailverify.ResultPage(locale, confirmed)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	// Both strings are our own, but they are escaped anyway: the day one of
	// them starts carrying a caller's purpose or address, the escaping is
	// already there rather than being the thing somebody forgot.
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="%s"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>body{font-family:system-ui,-apple-system,"Segoe UI",sans-serif;background:#f8fafc;color:#0f172a;display:grid;place-items:center;min-height:100vh;margin:0}
main{background:#fff;border:1px solid #e2e8f0;border-radius:12px;padding:40px;max-width:420px;text-align:center;box-shadow:0 1px 3px rgba(15,23,42,.08)}
h1{font-size:20px;margin:0 0 12px}p{color:#475569;font-size:14px;line-height:1.6;margin:0}</style>
</head><body><main><h1>%s</h1><p>%s</p></main></body></html>`,
		html.EscapeString(locale), html.EscapeString(title), html.EscapeString(title), html.EscapeString(body))
}

// handleEmailVerifyOverview is the settings screen in one request: what has
// been asked for, and whether the service that sends it is reachable.
func (s *Server) handleEmailVerifyOverview(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, convErr := strconv.Atoi(raw); convErr == nil {
			limit = parsed
		}
	}
	overview, err := s.emailVerify.Overview(r.Context(), tenantID, limit)
	if err != nil {
		emailVerifyError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, overview)
}
