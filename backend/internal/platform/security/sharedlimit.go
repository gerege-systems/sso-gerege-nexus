package security

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
	"github.com/redis/go-redis/v9"
)

// A per-process limiter divides by the number of processes.
//
// IPRateLimiter counts in memory, so five replicas behind one nginx gave a
// caller five times the budget the number in the code says, and a rollout gave
// everybody a fresh one. For the endpoints in front of a password, a citizen's
// phone, or somebody else's paid API, that is the difference between a limit
// and a suggestion.
//
// This counts in Redis instead, so the budget belongs to the deployment. It
// sits behind the local limiter rather than replacing it: the local one is free
// and stops a flood before it reaches the network, and this one is what makes
// the number mean what it says. Without REDIS_URL the local limiter is all
// there is, which is what the platform has always had.

// SharedLimiter counts requests per caller across every replica.
type SharedLimiter struct {
	client *redis.Client
	// window and limit are a fixed window rather than a token bucket: the bucket
	// belongs to the local limiter in front, which already smooths bursts. What
	// is wanted here is a ceiling that several processes agree on, and a counter
	// with an expiry is the cheapest thing that gives one — a single round trip,
	// no script, no clock shared between replicas.
	window time.Duration
	limit  int
	name   string
}

// NewSharedLimiter returns nil when no client is configured, and a nil
// *SharedLimiter is a working no-op — so a caller never has to ask whether
// Redis is present.
func NewSharedLimiter(client *redis.Client, name string, limit int, window time.Duration) *SharedLimiter {
	if client == nil || limit <= 0 {
		return nil
	}
	return &SharedLimiter{client: client, name: name, limit: limit, window: window}
}

// Allow reports whether this caller may proceed, and how long until they may.
//
// A Redis that cannot answer allows the request. The local limiter is still in
// front of it, and turning a cache outage into an inability to sign in would be
// a worse failure than a caller briefly getting a per-replica budget.
func (l *SharedLimiter) Allow(ctx context.Context, caller string) (bool, time.Duration) {
	if l == nil {
		return true, 0
	}
	// The window is part of the key, so an expiring counter never has to be
	// reset and two replicas cannot disagree about which window they are in.
	slot := time.Now().UnixNano() / int64(l.window)
	key := "gerege_nexus:rl:" + l.name + ":" + caller + ":" + strconv.FormatInt(slot, 10)

	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	pipe := l.client.TxPipeline()
	count := pipe.Incr(ctx, key)
	// Set on every call rather than only the first: an INCR that raced an expiry
	// would otherwise leave a counter with no TTL, and that caller would be
	// locked out until somebody noticed.
	pipe.Expire(ctx, key, l.window)
	if _, err := pipe.Exec(ctx); err != nil {
		slog.Warn("security: shared rate limiter is unavailable; falling back to the per-process one",
			"limiter", l.name, "error", err)
		return true, 0
	}
	if count.Val() > int64(l.limit) {
		return false, l.window
	}
	return true, 0
}

// SharedRateLimitMiddleware applies the deployment-wide budget after the
// per-process one has had its say.
func SharedRateLimitMiddleware(local *IPRateLimiter, shared *SharedLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			caller := ClientIP(r)
			if !local.GetLimiter(caller).Allow() {
				writeTooManyRequests(w, 0)
				return
			}
			if ok, retryAfter := shared.Allow(r.Context(), caller); !ok {
				writeTooManyRequests(w, retryAfter)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeTooManyRequests(w http.ResponseWriter, retryAfter time.Duration) {
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
	}
	httpx.Error(w, http.StatusTooManyRequests, "too many requests: rate limit exceeded, try again later")
}
