package platform

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
)

// The JIT preimage was once the digest plus a ":eid-only" suffix — 73 bytes
// against bcrypt's hard 72 — so every first-time eID sign-in died at account
// creation and the citizen read the library's complaint in the eID card.
func TestEIDLinkingDigestHashesUnderBcrypt(t *testing.T) {
	subjects := []string{"AA90010111", "CID-99887766", strings.Repeat("Ө", 64)}
	for _, subject := range subjects {
		digest := eidLinkingDigest("linking-key", subject)
		if len(digest) != 64 {
			t.Fatalf("digest for %q is %d bytes, want 64", subject, len(digest))
		}
		if len(digest) > 72 {
			t.Fatalf("digest for %q exceeds bcrypt's 72-byte input limit", subject)
		}
		if _, err := auth.HashPassword(digest); err != nil {
			t.Fatalf("bcrypt rejected the digest for %q: %v", subject, err)
		}
	}
}

// The digest is the account's identity across sign-ins, so it has to be stable
// for one subject and distinct between two.
func TestEIDLinkingDigestIsStableAndSubjectBound(t *testing.T) {
	if a, b := eidLinkingDigest("k", "AA90010111"), eidLinkingDigest("k", "AA90010111"); a != b {
		t.Errorf("same key and subject produced different digests: %s vs %s", a, b)
	}
	if a, b := eidLinkingDigest("k", "AA90010111"), eidLinkingDigest("k", "AA90010112"); a == b {
		t.Error("two subjects share a digest, so they would share an account")
	}
	if a, b := eidLinkingDigest("k1", "AA90010111"), eidLinkingDigest("k2", "AA90010111"); a == b {
		t.Error("the linking key does not affect the digest")
	}
}

// The whole point of splitting EID_LINKING_KEY out of EID_RP_SECRET is that
// rotating the API credential must not move an existing citizen's account. The
// digest is both their lookup key and their password preimage, so "moved" here
// means locked out, not inconvenienced.
func TestEIDLinkingKeySurvivesAPISecretRotation(t *testing.T) {
	const subject = "AA90010111"

	// Before: no dedicated key, so the RP secret is the linking key. This is
	// the behaviour every deployment that never sets EID_LINKING_KEY keeps.
	t.Setenv("EID_LINKING_KEY", "")
	t.Setenv("EID_RP_SECRET", "rp-secret-v1")
	if got, want := eidLinkingKey(), "rp-secret-v1"; got != want {
		t.Fatalf("without EID_LINKING_KEY the RP secret must be the linking key: got %q, want %q", got, want)
	}
	before := eidLinkingDigest(eidLinkingKey(), subject)

	// Pin the old secret as the linking key, then rotate the API credential.
	t.Setenv("EID_LINKING_KEY", "rp-secret-v1")
	t.Setenv("EID_RP_SECRET", "rp-secret-v2-rotated")
	if after := eidLinkingDigest(eidLinkingKey(), subject); after != before {
		t.Errorf("rotating EID_RP_SECRET moved the account: %s -> %s", before, after)
	}

	// And without pinning it, rotation does move the account — the failure this
	// separation exists to prevent.
	t.Setenv("EID_LINKING_KEY", "")
	if after := eidLinkingDigest(eidLinkingKey(), subject); after == before {
		t.Error("rotation left the digest unchanged even with no linking key pinned, so this test proves nothing")
	}
}

func TestEIDLinkingKeyPrecedenceAndBlankHandling(t *testing.T) {
	// A set key wins outright.
	t.Setenv("EID_LINKING_KEY", "dedicated")
	t.Setenv("EID_RP_SECRET", "rp")
	if got := eidLinkingKey(); got != "dedicated" {
		t.Errorf("EID_LINKING_KEY must take precedence, got %q", got)
	}

	// Whitespace-only is an operator slip, not a key: fall back rather than
	// hash under a value nobody chose.
	t.Setenv("EID_LINKING_KEY", "   ")
	if got := eidLinkingKey(); got != "rp" {
		t.Errorf("whitespace-only EID_LINKING_KEY must fall back, got %q", got)
	}

	// A real key keeps its padding, because it has to reproduce the previous
	// secret byte for byte.
	t.Setenv("EID_LINKING_KEY", " padded ")
	if got := eidLinkingKey(); got != " padded " {
		t.Errorf("EID_LINKING_KEY must not be trimmed, got %q", got)
	}

	// Neither set is a configuration error the caller has to report.
	t.Setenv("EID_LINKING_KEY", "")
	t.Setenv("EID_RP_SECRET", "")
	if got := eidLinkingKey(); got != "" {
		t.Errorf("with neither set the key must be empty, got %q", got)
	}
}

func TestReportSignInFailureHidesInternalReasons(t *testing.T) {
	rec := httptest.NewRecorder()
	reportSignInFailure(rec, errors.New("bcrypt: password length exceeds 72 bytes"))
	if rec.Code != 500 {
		t.Errorf("internal failure answered %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "bcrypt") {
		t.Errorf("internal reason reached the caller: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	reportSignInFailure(rec, signInError{"eID identity is verified but account provisioning is disabled"})
	if rec.Code != 403 {
		t.Errorf("caller-facing failure answered %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "account provisioning is disabled") {
		t.Errorf("caller-facing reason was suppressed: %s", rec.Body.String())
	}
}
