package security_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/cache"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/security"
	"github.com/google/uuid"
)

// A nil limiter is what every caller gets when no Redis is configured, and it
// has to be usable without anybody checking for it first.
func TestANilSharedLimiterAllows(t *testing.T) {
	var absent *security.SharedLimiter
	if ok, _ := absent.Allow(context.Background(), "1.2.3.4"); !ok {
		t.Error("a limiter that is not configured refused a request")
	}
	if security.NewSharedLimiter(nil, "x", 10, time.Minute) != nil {
		t.Error("a limiter was built without a client")
	}
}

func sharedLimiter(t *testing.T, name string, limit int, window time.Duration) *security.SharedLimiter {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL is not set")
	}
	client := cache.Dial(url)
	if client == nil {
		t.Fatal("Dial returned no client")
	}
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skipf("Redis is not answering: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return security.NewSharedLimiter(client, name, limit, window)
}

// The point of counting in Redis: two processes share one budget. Two limiters
// with the same name are what two replicas are.
func TestTwoReplicasShareOneBudget(t *testing.T) {
	name := "test-" + uuid.NewString()[:8]
	first := sharedLimiter(t, name, 5, time.Minute)
	second := sharedLimiter(t, name, 5, time.Minute)
	caller := uuid.NewString()
	ctx := context.Background()

	// Three on one replica, two on the other: five in total, all allowed.
	for i := range 3 {
		if ok, _ := first.Allow(ctx, caller); !ok {
			t.Fatalf("first replica refused request %d of its first three", i+1)
		}
	}
	for i := range 2 {
		if ok, _ := second.Allow(ctx, caller); !ok {
			t.Fatalf("second replica refused request %d; the budget was not shared", i+1)
		}
	}

	// The sixth is over the limit whichever replica it arrives at.
	if ok, retryAfter := second.Allow(ctx, caller); ok {
		t.Error("the shared budget did not run out")
	} else if retryAfter <= 0 {
		t.Error("a refusal carried no Retry-After")
	}
}

// One caller running out must not spend anybody else's budget.
func TestOneCallerRunningOutDoesNotAffectAnother(t *testing.T) {
	limiter := sharedLimiter(t, "test-"+uuid.NewString()[:8], 2, time.Minute)
	ctx := context.Background()
	exhausted, innocent := uuid.NewString(), uuid.NewString()

	for range 3 {
		limiter.Allow(ctx, exhausted)
	}
	if ok, _ := limiter.Allow(ctx, exhausted); ok {
		t.Fatal("the exhausted caller was still allowed")
	}
	if ok, _ := limiter.Allow(ctx, innocent); !ok {
		t.Error("a different caller was refused")
	}
}

// A counter that lost its expiry would lock a caller out until somebody noticed.
func TestTheCounterAlwaysCarriesAnExpiry(t *testing.T) {
	limiter := sharedLimiter(t, "test-"+uuid.NewString()[:8], 1, 2*time.Second)
	ctx := context.Background()
	caller := uuid.NewString()

	if ok, _ := limiter.Allow(ctx, caller); !ok {
		t.Fatal("the first request was refused")
	}
	if ok, _ := limiter.Allow(ctx, caller); ok {
		t.Fatal("the second request was allowed past a limit of one")
	}
	// The window is part of the key, so the next one is a different counter.
	time.Sleep(2100 * time.Millisecond)
	if ok, _ := limiter.Allow(ctx, caller); !ok {
		t.Error("the caller was still refused after the window had passed")
	}
}
