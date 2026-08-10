package security_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/security"
	"golang.org/x/time/rate"
)

func TestHeadersMiddleware(t *testing.T) {
	handler := security.HeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("expected X-Frame-Options: DENY")
	}
}

func TestIsValidSlug(t *testing.T) {
	if !security.IsValidSlug("contacts") {
		t.Errorf("expected 'contacts' to be valid slug")
	}
	if !security.IsValidSlug("io-example-inventory") {
		t.Errorf("expected 'io-example-inventory' to be valid slug")
	}
	if security.IsValidSlug("../../../etc/passwd") {
		t.Errorf("expected path traversal slug to fail validation")
	}
	if security.IsValidSlug("<script>") {
		t.Errorf("expected XSS injection slug to fail validation")
	}
}

func TestSafeCORSOrigins(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		want  []string
		unset bool
	}{
		{name: "unset falls back to local development", unset: true,
			want: []string{"http://localhost:3000", "http://127.0.0.1:3000"}},
		{name: "single origin", env: "https://nexus.gov.mn",
			want: []string{"https://nexus.gov.mn"}},
		// The reason this function exists: a list written with spaces after the
		// commas used to yield origins with a leading space, which match no
		// Origin header a browser ever sends.
		{name: "spaces after the commas are not part of the origin",
			env:  "https://a.gov.mn, https://b.gov.mn ,\thttps://c.gov.mn",
			want: []string{"https://a.gov.mn", "https://b.gov.mn", "https://c.gov.mn"}},
		{name: "empty entries are dropped", env: "https://a.gov.mn,,https://b.gov.mn,",
			want: []string{"https://a.gov.mn", "https://b.gov.mn"}},
		{name: "a value of only separators falls back rather than allowing nothing",
			env:  " , ",
			want: []string{"http://localhost:3000", "http://127.0.0.1:3000"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.unset {
				t.Setenv("ALLOWED_ORIGINS", "")
			} else {
				t.Setenv("ALLOWED_ORIGINS", tc.env)
			}
			got := security.SafeCORSOrigins()
			if len(got) != len(tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %q, want %q", got, tc.want)
				}
			}
		})
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := security.NewIPRateLimiter(rate.Limit(2), 2) // 2 requests allowed
	middleware := security.RateLimitMiddleware(limiter)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/auth/login", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	// Request 1: OK
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec1.Code)
	}

	// Request 2: OK
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}

	// Request 3: 429 Too Many Requests
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req)
	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d", rec3.Code)
	}
}

func TestClientIPUsesTrustedProxyBoundary(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.10:4321"
	req.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.7")
	req.Header.Set("X-Real-IP", "192.0.2.9")

	t.Setenv("TRUST_PROXY_HEADERS", "")
	if got := security.ClientIP(req); got != "10.0.0.10" {
		t.Fatalf("untrusted headers: got %q", got)
	}
	t.Setenv("TRUST_PROXY_HEADERS", "true")
	if got := security.ClientIP(req); got != "192.0.2.9" {
		t.Fatalf("X-Real-IP: got %q", got)
	}
	req.Header.Del("X-Real-IP")
	if got := security.ClientIP(req); got != "203.0.113.7" {
		t.Fatalf("right-most XFF: got %q", got)
	}
}
