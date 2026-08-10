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
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/audit"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/config"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/dan"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/eid"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
)

func (s *Server) handleEIDStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CallbackURL string `json:"callbackUrl"`
	}
	if r.Body != nil {
		_ = decodeLimitedJSON(r, &req, 8<<10)
	}
	callback, err := validEIDCallback(req.CallbackURL)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid eID callback URL")
		return
	}
	started, err := s.eidSvc.StartDeviceLink(r.Context(), callback)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "eID Mongolia session could not be started")
		return
	}
	httpx.JSON(w, http.StatusOK, started)
}

func (s *Server) handleEIDStartByNationalID(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NationalID  string `json:"national_id"`
		CallbackURL string `json:"callbackUrl"`
	}
	if decodeLimitedJSON(r, &req, 8<<10) != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	callback, err := validEIDCallback(req.CallbackURL)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid eID callback URL")
		return
	}
	started, err := s.eidSvc.StartByNationalID(r.Context(), req.NationalID, callback)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "Регистрийн дугаар олдсонгүй эсвэл eID апп-д бүртгэлгүй байна")
		return
	}
	httpx.JSON(w, http.StatusOK, started)
}

func validEIDCallback(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	callback, err := url.Parse(raw)
	if err != nil || callback.User != nil || (callback.Scheme != "https" && (config.IsProduction() || callback.Scheme != "http")) {
		return "", errors.New("invalid callback")
	}
	publicOrigin := strings.TrimSpace(os.Getenv("PUBLIC_ORIGIN"))
	if publicOrigin == "" {
		publicOrigin = "http://localhost:3000"
	}
	origin, err := url.Parse(publicOrigin)
	if err != nil || !strings.EqualFold(callback.Host, origin.Host) || callback.Path != "/auth/eid/callback" {
		return "", errors.New("callback not allowed")
	}
	return callback.String(), nil
}

func (s *Server) handleEIDPoll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if decodeLimitedJSON(r, &req, 8<<10) != nil || strings.TrimSpace(req.SessionID) == "" {
		httpx.Error(w, http.StatusBadRequest, "session_id is required")
		return
	}
	result, err := s.eidSvc.Poll(r.Context(), req.SessionID)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "eID Mongolia session check failed")
		return
	}
	if result.State != "COMPLETE" {
		httpx.JSON(w, http.StatusOK, result)
		return
	}
	if result.Identity == nil || !result.Identity.VerifiedStatus {
		httpx.Error(w, http.StatusUnauthorized, "eID identity verification failed")
		return
	}
	userID, tenantID, err := s.resolveOrProvisionEIDUser(r.Context(), result.Identity)
	if err != nil {
		reportSignInFailure(w, err)
		return
	}
	s.linkEIDIdentity(r.Context(), userID, result.Identity)
	token, expiresAt, err := s.issueSession(r, userID, tenantID, "eid-app")
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to establish session")
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)
	audit.Record(r.Context(), tenantID, userID, "auth.eid_app_login_success", "eid", map[string]any{"verified": true, "method": "eid-app"})
	httpx.JSON(w, http.StatusOK, map[string]any{"state": result.State, "expires_at": expiresAt, "identity": result.Identity})
}

func (s *Server) handleEIDLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code        string         `json:"code"`
		RedirectURI string         `json:"redirect_uri"`
		RegNumber   string         `json:"reg_number"`
		OTPCode     string         `json:"otp_code"`
		AuthMethod  eid.AuthMethod `json:"auth_method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}

	var identity *eid.EIDIdentity
	var err error
	if req.Code != "" {
		identity, err = s.eidSvc.ExchangeCode(r.Context(), req.Code, req.RedirectURI)
	} else if req.RegNumber != "" {
		identity, err = s.eidSvc.AuthenticateWithMethod(r.Context(), req.RegNumber, req.OTPCode, req.AuthMethod)
	} else {
		httpx.Error(w, http.StatusBadRequest, "missing authorization code or registration number")
		return
	}

	// err may be nil while identity is nil — calling err.Error() unguarded
	// panicked the request goroutine.
	if err != nil || identity == nil {
		msg := "E-ID verification failed"
		if err != nil {
			msg = "E-ID verification failed: " + err.Error()
		}
		httpx.Error(w, http.StatusUnauthorized, msg)
		return
	}

	userID, tenantID, err := s.resolveOrProvisionEIDUser(r.Context(), identity)
	if err != nil {
		reportSignInFailure(w, err)
		return
	}
	s.linkEIDIdentity(r.Context(), userID, identity)

	token, expiresAt, err := s.issueSession(r, userID, tenantID, "eid")
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to establish session")
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)

	audit.Record(r.Context(), tenantID, userID, "auth.eid_login_success", "eid", map[string]any{
		"reg_number": identity.RegNumber,
		"civil_id":   identity.CivilID,
	})

	claims, _ := s.sessions.Resolve(r.Context(), token)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"expires_at": expiresAt,
		"identity":   identity,
		"user": map[string]any{
			"id":        userID,
			"tenant_id": tenantID,
			"name":      identity.FirstName + " " + identity.LastName,
			"email":     claims.Email,
			"is_admin":  claims.IsAdmin,
		},
	})
}

func (s *Server) handleDANLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DANToken  string `json:"dan_token"`
		RegNumber string `json:"reg_number"`
		OTPCode   string `json:"otp_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}

	var profile *dan.DANProfile
	var err error
	if req.DANToken != "" {
		profile, err = s.danSvc.VerifyDANToken(r.Context(), req.DANToken)
	} else if req.RegNumber != "" {
		profile, err = s.danSvc.AuthenticateDANCitizen(r.Context(), req.RegNumber, req.OTPCode)
	} else {
		httpx.Error(w, http.StatusBadRequest, "missing dan_token or registration number")
		return
	}

	if err != nil || profile == nil {
		msg := "dan.gerege.mn verification failed"
		if err != nil {
			msg = "dan.gerege.mn verification failed: " + err.Error()
		}
		httpx.Error(w, http.StatusUnauthorized, msg)
		return
	}

	identity := &eid.EIDIdentity{
		CivilID: profile.CivilID, RegNumber: profile.RegNumber, FirstName: profile.FirstName, LastName: profile.LastName,
		VerifiedStatus: true,
	}
	userID, tenantID, err := s.resolveOrProvisionEIDUser(r.Context(), identity)
	if err != nil {
		reportSignInFailure(w, err)
		return
	}
	s.linkEIDIdentity(r.Context(), userID, identity)

	token, expiresAt, err := s.issueSession(r, userID, tenantID, "dan")
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to establish session")
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)

	audit.Record(r.Context(), tenantID, userID, "auth.dan_gerege_login_success", "dan", map[string]any{
		"reg_number":  profile.RegNumber,
		"dan_session": profile.DANSessionID,
	})

	claims, _ := s.sessions.Resolve(r.Context(), token)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"expires_at":  expiresAt,
		"dan_profile": profile,
		"user": map[string]any{
			"id":        userID,
			"tenant_id": tenantID,
			"name":      profile.FirstName + " " + profile.LastName,
			"email":     claims.Email,
			"is_admin":  claims.IsAdmin,
		},
	})
}

func (s *Server) handleXYPCitizenQuery(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	var req struct {
		RegNumber string `json:"reg_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RegNumber == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid registration number")
		return
	}

	info, err := s.geregeSvc.GetCitizenInfo(r.Context(), req.RegNumber)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "XYP citizen query failed: "+err.Error())
		return
	}

	claims, _ := auth.UserFromContext(r.Context())
	audit.Record(r.Context(), tenantID, claims.UserID, "xyp.citizen_queried", "xyp", map[string]any{"reg_number": req.RegNumber})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

func (s *Server) handleXYPCompanyQuery(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	var req struct {
		CompanyReg string `json:"company_reg"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CompanyReg == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid company registration number")
		return
	}

	info, err := s.geregeSvc.GetCompanyInfo(r.Context(), req.CompanyReg)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "XYP company query failed: "+err.Error())
		return
	}

	claims, _ := auth.UserFromContext(r.Context())
	audit.Record(r.Context(), tenantID, claims.UserID, "xyp.company_queried", "xyp", map[string]any{"company_reg": req.CompanyReg})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}
