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
	const docID = "3f1b9c62-2f1a-4a1c-9d3e-8b7a5c4e1d20"

	for _, path := range []string{
		"/documents/" + docID + "/sign",
		"/documents/" + docID + "/sign/dan",
		"/documents/" + docID + "/sign/eid/start",
		"/documents/" + docID + "/sign/eid/poll",
		"/documents/" + docID + "/reject",
	} {
		if got := appRequestPermission("io.example.documents", "POST", path); got != "documents.sign" {
			t.Errorf("%s: got %q, want documents.sign", path, got)
		}
	}

	// Reading the ledger is an ordinary read, even though the path starts with
	// the same five letters as the decision routes.
	if got := appRequestPermission("io.example.documents", "GET", "/documents/"+docID+"/signatures"); got != "documents.read" {
		t.Errorf("signatures: got %q, want documents.read", got)
	}

	// Creating and listing keep the ordinary model-level rights.
	if got := appRequestPermission("io.example.documents", "POST", "/documents"); got != "documents.manage" {
		t.Errorf("create: got %q, want documents.manage", got)
	}
	if got := appRequestPermission("io.example.documents", "GET", "/documents"); got != "documents.read" {
		t.Errorf("list: got %q, want documents.read", got)
	}

	// The PDF e-sign app runs its own checks and must not be caught by the
	// Every other documents route, so the split stays where it was put. Routing a
	// document and configuring a type are authoring, not approving: a clerk may draft
	// and route what they cannot sign, and an approver may sign what they cannot
	// configure. Reading a document's own chain is an ordinary read.
	for _, tc := range []struct{ method, path, want string }{
		{"POST", "/documents/" + docID + "/route", "documents.manage"},
		{"GET", "/documents/" + docID + "/steps", "documents.read"},
		{"GET", "/documents/templates", "documents.read"},
		{"POST", "/documents/templates", "documents.manage"},
		{"PUT", "/documents/templates/" + docID, "documents.manage"},
		{"DELETE", "/documents/templates/" + docID, "documents.manage"},
		{"POST", "/documents/templates/" + docID + "/use", "documents.manage"},
		{"GET", "/documents/workflows", "documents.read"},
		{"PUT", "/documents/workflows/CONTRACT", "documents.manage"},
		{"GET", "/documents/policies", "documents.read"},
		{"PUT", "/documents/policies/CONTRACT", "documents.manage"},
		{"GET", "/documents/retention", "documents.read"},
		{"PUT", "/documents/retention/CONTRACT", "documents.manage"},
	} {
		if got := appRequestPermission("io.example.documents", tc.method, tc.path); got != tc.want {
			t.Errorf("%s %s: got %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}

	// documents rule even though its route also ends in /sign.
	if got := appRequestPermission("io.example.esign", "POST", "/esign/documents/"+docID+"/sign"); got != "" {
		t.Errorf("esign: got %q, want no central check", got)
	}
}
