package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/integration"
)

func TestIntegrationManagerList(t *testing.T) {
	mgr := integration.NewManager()

	list := mgr.List()
	if len(list) < 2 {
		t.Fatalf("expected at least 2 default integrations, got %d", len(list))
	}
}

func TestIntegrationManagerRegisterAndDispatch(t *testing.T) {
	mgr := integration.NewManager()

	mgr.Register(&IntegrationConfigTest{
		ID:        "int_test_webhook",
		Name:      "Test Webhook",
		Type:      "webhook",
		TargetURL: "https://httpbin.org/post",
	})

	err := mgr.DispatchEvent(context.Background(), integration.EventPayload{
		EventID:   "evt_1001",
		EventType: "contact.created",
		TenantID:  "00000000-0000-0000-0000-000000000001",
		Timestamp: time.Now(),
		Data:      map[string]any{"name": "Test User"},
	})

	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
}

type IntegrationConfigTest = integration.IntegrationConfig
