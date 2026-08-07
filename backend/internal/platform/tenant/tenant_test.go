package tenant_test

import (
	"context"
	"testing"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
)

func TestTenantContextIsolation(t *testing.T) {
	ctx := context.Background()

	_, err := tenant.FromContext(ctx)
	if err == nil {
		t.Fatal("expected error when tenant ID is missing from context")
	}

	ctxWithTenant := tenant.WithTenantID(ctx, "tenant-123")
	tenantID, err := tenant.FromContext(ctxWithTenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenantID != "tenant-123" {
		t.Fatalf("expected tenant-123, got %s", tenantID)
	}
}
