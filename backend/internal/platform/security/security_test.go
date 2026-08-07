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
