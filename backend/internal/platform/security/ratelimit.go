package security

import (
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/async"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
	"golang.org/x/time/rate"
)

type IPRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	limiter := &IPRateLimiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    b,
	}
	async.Go("rate-limiter-cleanup", limiter.cleanupVisitors)
	return limiter
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	v, exists := i.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(i.rate, i.burst)
		i.visitors[ip] = &visitor{limiter, time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

func (i *IPRateLimiter) cleanupVisitors() {
	for {
		time.Sleep(3 * time.Minute)
		i.mu.Lock()
		for ip, v := range i.visitors {
			if time.Since(v.lastSeen) > 5*time.Minute {
				delete(i.visitors, ip)
			}
		}
		i.mu.Unlock()
	}
}

// RateLimitMiddleware returns an HTTP middleware that throttles request bursts per IP.
func RateLimitMiddleware(limiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r)
			l := limiter.GetLimiter(ip)
			if !l.Allow() {
				httpx.Error(w, http.StatusTooManyRequests, "too many requests: rate limit exceeded, try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP resolves the caller address used for rate limiting and audit logs.
//
// X-Forwarded-For / X-Real-IP are attacker-controlled unless a trusted proxy
// rewrites them, so honouring them unconditionally lets anyone bypass the login
// rate limiter by varying a header. They are only consulted when the deployment
// declares TRUST_PROXY_HEADERS=true.
func ClientIP(r *http.Request) string {
	if trustProxyHeaders() {
		if xrip := validIP(r.Header.Get("X-Real-IP")); xrip != "" {
			return xrip
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			// A trusted edge proxy appends the address it received the request
			// from. The left-most values can already be supplied by the caller;
			// the right-most value is therefore the only safe single-proxy hop.
			for idx := len(parts) - 1; idx >= 0; idx-- {
				if ip := validIP(parts[idx]); ip != "" {
					return ip
				}
			}
		}
	}

	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func validIP(raw string) string {
	ip := strings.TrimSpace(raw)
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

func trustProxyHeaders() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TRUST_PROXY_HEADERS"))) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}
