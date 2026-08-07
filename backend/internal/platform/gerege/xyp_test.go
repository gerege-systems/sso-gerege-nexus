package gerege_test

import (
	"context"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/gerege"
)

func TestGeregeMockCitizenQuery(t *testing.T) {
	svc := gerege.NewGeregeService()

	info, err := svc.GetCitizenInfo(context.Background(), "AA90010111")
	if err != nil {
		t.Fatalf("unexpected error during mock Gerege citizen query: %v", err)
	}

	if info.RegNumber != "AA90010111" {
		t.Errorf("expected RegNumber AA90010111, got %s", info.RegNumber)
	}
	if !info.Verified {
		t.Errorf("expected verified = true")
	}
}

func TestGeregeMockCompanyQuery(t *testing.T) {
	svc := gerege.NewGeregeService()

	company, err := svc.GetCompanyInfo(context.Background(), "5589412")
	if err != nil {
		t.Fatalf("unexpected error during mock Gerege company query: %v", err)
	}

	if company.Name != "Гэрэгэ Системс ХХК" {
		t.Errorf("unexpected company name: %s", company.Name)
	}
	if !company.VatPayer {
		t.Errorf("expected vat_payer = true")
	}
}
