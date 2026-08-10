package config

import "testing"

func TestValidateProduction(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	for _, name := range []string{"DATABASE_URL", "PUBLIC_ORIGIN", "ALLOWED_ORIGINS", "SSO_DEFAULT_CLIENT_SECRET"} {
		t.Setenv(name, "set")
	}
	t.Setenv("PUBLIC_ORIGIN", "https://nexus.example")
	if err := ValidateProduction(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	t.Setenv("PUBLIC_ORIGIN", "http://nexus.example")
	if err := ValidateProduction(); err == nil {
		t.Fatal("expected insecure origin rejection")
	}
}
