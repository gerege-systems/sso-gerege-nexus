/*
 * Gerege SSO
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Package integration provides an asynchronous Webhook Event Dispatcher and
 * external system REST Connector Manager with HMAC-SHA256 signature signing.
 */

package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type ConnectorStatus string

const (
	StatusActive   ConnectorStatus = "ACTIVE"
	StatusInactive ConnectorStatus = "INACTIVE"
	StatusError    ConnectorStatus = "ERROR"
)

type IntegrationConfig struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"` // e.g. "webhook", "government", "payment", "custom_rest"
	TargetURL  string            `json:"target_url"`
	SecretKey  string            `json:"secret_key,omitempty"`
	Status     ConnectorStatus   `json:"status"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	LastPingAt time.Time         `json:"last_ping_at"`
}

type EventPayload struct {
	EventID   string         `json:"event_id"`
	EventType string         `json:"event_type"` // e.g. "contact.created", "stock.adjusted", "order.placed"
	TenantID  string         `json:"tenant_id"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

type Manager struct {
	mu           sync.RWMutex
	integrations map[string]*IntegrationConfig
	httpClient   *http.Client
}

func NewManager() *Manager {
	m := &Manager{
		integrations: make(map[string]*IntegrationConfig),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// Register default system connectors
	m.Register(&IntegrationConfig{
		ID:         "int_gerege_xyp",
		Name:       "Gerege XYP State Exchange",
		Type:       "government",
		TargetURL:  "https://xyp.gerege.mn/api/v1",
		Status:     StatusActive,
		LastPingAt: time.Now(),
	})

	m.Register(&IntegrationConfig{
		ID:         "int_eid_sso",
		Name:       "Gerege E-ID Digital Identity SSO",
		Type:       "government",
		TargetURL:  "https://eid.gerege.mn/api/v1",
		Status:     StatusActive,
		LastPingAt: time.Now(),
	})

	return m
}

func (m *Manager) Register(cfg *IntegrationConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg.Status == "" {
		cfg.Status = StatusActive
	}
	cfg.LastPingAt = time.Now()
	m.integrations[cfg.ID] = cfg
}

func (m *Manager) List() []*IntegrationConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*IntegrationConfig, 0, len(m.integrations))
	for _, cfg := range m.integrations {
		list = append(list, cfg)
	}
	return list
}

func (m *Manager) DispatchEvent(ctx context.Context, payload EventPayload) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event payload: %w", err)
	}

	for _, cfg := range m.integrations {
		if cfg.Status != StatusActive || cfg.TargetURL == "" || cfg.Type != "webhook" {
			continue
		}

		go func(targetURL, secret string) {
			req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(bodyBytes))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			// X-ERP-* are legacy compatibility header names, kept through both
			// the Gerege Nexus and Gerege SSO rebrands. Subscribers read these
			// exact names and verify the signature against them; renaming would
			// break every existing endpoint silently. An SSO-named alias would
			// need dual-emission and a deprecation window, not a rename.
			req.Header.Set("X-ERP-Event", payload.EventType)

			if secret != "" {
				mac := hmac.New(sha256.New, []byte(secret))
				mac.Write(bodyBytes)
				sig := hex.EncodeToString(mac.Sum(nil))
				req.Header.Set("X-ERP-Signature", sig)
			}

			resp, err := m.httpClient.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		}(cfg.TargetURL, cfg.SecretKey)
	}

	return nil
}
