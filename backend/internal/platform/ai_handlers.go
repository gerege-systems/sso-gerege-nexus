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
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/ai"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleAICopilot(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Prompt == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid prompt")
		return
	}

	res, err := s.copilotSvc.Query(r.Context(), ai.CopilotRequest{
		Prompt:   req.Prompt,
		TenantID: tenantID,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleAIChat(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	var req ai.CopilotRequest
	if err := decodeLimitedJSON(r, &req, 1<<20); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid AI request")
		return
	}
	req.TenantID = tenantID
	res, err := s.copilotSvc.Query(r.Context(), req)
	if err != nil {
		httpx.Error(w, aiStatus(err), err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (s *Server) handleAISTT(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Audio ai.Audio `json:"audio"`
	}
	if err := decodeLimitedJSON(r, &req, 1<<20); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid audio request")
		return
	}
	text, err := s.copilotSvc.Transcribe(r.Context(), req.Audio)
	if err != nil {
		httpx.Error(w, aiStatus(err), err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"text": text})
}

func (s *Server) handleAITTS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := decodeLimitedJSON(r, &req, 16<<10); err != nil || req.Text == "" {
		httpx.Error(w, http.StatusBadRequest, "text is required")
		return
	}
	audio, err := s.copilotSvc.Speak(r.Context(), req.Text)
	if err != nil {
		httpx.Error(w, aiStatus(err), err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, audio)
}

func (s *Server) handleAITranslate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text   string    `json:"text"`
		Audio  *ai.Audio `json:"audio"`
		Target string    `json:"target_lang"`
		Speak  bool      `json:"speak"`
	}
	if err := decodeLimitedJSON(r, &req, 1<<20); err != nil || req.Target == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid translation request")
		return
	}
	if req.Text == "" && req.Audio != nil {
		var err error
		req.Text, err = s.copilotSvc.Transcribe(r.Context(), *req.Audio)
		if err != nil {
			httpx.Error(w, aiStatus(err), err.Error())
			return
		}
	}
	translated, err := s.copilotSvc.Translate(r.Context(), req.Text, req.Target)
	if err != nil {
		httpx.Error(w, aiStatus(err), err.Error())
		return
	}
	result := map[string]any{"source_text": req.Text, "translated": translated}
	if req.Speak {
		if sound, e := s.copilotSvc.Speak(r.Context(), translated); e == nil {
			result["audio"] = sound
		}
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (s *Server) handleAIListPrompts(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT prompt_key,content,active,tenant_id IS NULL FROM ai_prompts WHERE tenant_id IS NULL OR tenant_id=$1 ORDER BY prompt_key,tenant_id NULLS FIRST`, tenantID)
	if err != nil {
		httpx.Error(w, 500, "failed to load AI prompts")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var key, content string
		var active, global bool
		if err := rows.Scan(&key, &content, &active, &global); err != nil {
			httpx.Error(w, 500, "failed to read AI prompts")
			return
		}
		items = append(items, map[string]any{"key": key, "content": content, "active": active, "global": global})
	}
	if err := rows.Err(); err != nil {
		httpx.Error(w, 500, "failed to read AI prompts")
		return
	}
	httpx.JSON(w, 200, items)
}
func (s *Server) handleAIUpdatePrompt(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	key := chi.URLParam(r, "key")
	if key != "scope" && key != "instructions" {
		httpx.Error(w, 400, "invalid prompt key")
		return
	}
	var req struct {
		Content string `json:"content"`
		Active  bool   `json:"active"`
	}
	if decodeLimitedJSON(r, &req, 32<<10) != nil || strings.TrimSpace(req.Content) == "" {
		httpx.Error(w, 400, "content is required")
		return
	}
	_, err := s.db.Exec(r.Context(), `INSERT INTO ai_prompts(tenant_id,prompt_key,content,active) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,prompt_key) DO UPDATE SET content=EXCLUDED.content,active=EXCLUDED.active,updated_at=NOW()`, tenantID, key, req.Content, req.Active)
	if err != nil {
		httpx.Error(w, 500, "failed to save AI prompt")
		return
	}
	httpx.JSON(w, 200, map[string]string{"status": "saved"})
}
func (s *Server) handleAIListKnowledge(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT id,title,content,source_url,updated_at FROM ai_knowledge WHERE tenant_id=$1 ORDER BY updated_at DESC LIMIT 100`, tenantID)
	if err != nil {
		httpx.Error(w, 500, "failed to load knowledge")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, title, content, url string
		var updated time.Time
		if err := rows.Scan(&id, &title, &content, &url, &updated); err != nil {
			httpx.Error(w, 500, "failed to read knowledge")
			return
		}
		items = append(items, map[string]any{"id": id, "title": title, "content": content, "source_url": url, "updated_at": updated})
	}
	if err := rows.Err(); err != nil {
		httpx.Error(w, 500, "failed to read knowledge")
		return
	}
	httpx.JSON(w, 200, items)
}
func (s *Server) handleAICreateKnowledge(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	var req struct {
		Title     string `json:"title"`
		Content   string `json:"content"`
		SourceURL string `json:"source_url"`
	}
	if decodeLimitedJSON(r, &req, 256<<10) != nil || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		httpx.Error(w, 400, "title and content are required")
		return
	}
	var id string
	err := s.db.QueryRow(r.Context(), `INSERT INTO ai_knowledge(tenant_id,title,content,source_url) VALUES($1,$2,$3,$4) RETURNING id`, tenantID, req.Title, req.Content, req.SourceURL).Scan(&id)
	if err != nil {
		httpx.Error(w, 500, "failed to save knowledge")
		return
	}
	httpx.JSON(w, 201, map[string]string{"id": id})
}

func decodeLimitedJSON(r *http.Request, dst any, max int64) error {
	return json.NewDecoder(io.LimitReader(r.Body, max)).Decode(dst)
}
func aiStatus(error) int { return http.StatusBadGateway }

func (s *Server) handleAIForecast(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	forecast, err := s.forecaster.AnalyzeTenantStock(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to generate forecast")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(forecast)
}
