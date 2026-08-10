package platform

import "testing"

func TestRoleCodeValidation(t *testing.T) {
	valid := []string{"admin", "sales_manager", "inventory.read"}
	invalid := []string{"A", "Admin", " has-space", "x/owner", "-admin"}
	for _, v := range valid {
		if !roleCodePattern.MatchString(v) {
			t.Errorf("expected %q valid", v)
		}
	}
	for _, v := range invalid {
		if roleCodePattern.MatchString(v) {
			t.Errorf("expected %q invalid", v)
		}
	}
}

func TestAppRequestPermission(t *testing.T) {
	if got := appRequestPermission("io.example.contacts", "GET", "/contacts"); got != "contacts.read" {
		t.Fatalf("got %q", got)
	}
	if got := appRequestPermission("io.example.contacts", "POST", "/contacts"); got != "contacts.manage" {
		t.Fatalf("got %q", got)
	}
	if got := appRequestPermission("io.example.gov_services", "POST", "/gov/requests"); got != "" {
		t.Fatalf("government workflow must keep action-level checks, got %q", got)
	}
}

func TestValidEIDCallback(t *testing.T) {
	// This repository's own origin. The neighbour it shares a host with answers
	// on nexus.gerege.mn; naming that one here would read as if this stack
	// deployed there.
	t.Setenv("PUBLIC_ORIGIN", "https://sso.gerege.mn")
	t.Setenv("ENVIRONMENT", "production")
	if got, err := validEIDCallback("https://sso.gerege.mn/auth/eid/callback"); err != nil || got == "" {
		t.Fatalf("expected callback to be accepted: %q, %v", got, err)
	}
	for _, raw := range []string{"http://sso.gerege.mn/auth/eid/callback", "https://evil.example/auth/eid/callback", "https://sso.gerege.mn/login"} {
		if _, err := validEIDCallback(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}

func TestSigningDocumentsNeedsItsOwnPermission(t *testing.T) {
	// Documents and the government workflow perform explicit permission checks
	// at route registration/handler level. URL text no longer decides authority.
	for _, appID := range []string{"io.example.documents", "io.example.gov_services", "io.example.esign"} {
		if got := appRequestPermission(appID, "POST", "/anything/sign/reject"); got != "" {
			t.Errorf("%s: got central permission %q", appID, got)
		}
	}
}
