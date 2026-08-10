package cache_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/cache"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/memo"
)

// A bus with no Redis is a supported deployment, not a broken one: it has to
// keep invalidating locally, because that is what a single-replica install
// relies on.
func TestALocalBusStillDropsEntriesHere(t *testing.T) {
	bus := cache.NewBus(context.Background(), nil)
	entries := memo.New[string](time.Minute)
	entries.Put(memo.Key("tenant-a", "user-1"), "held")
	bus.Register("grants", entries)

	if bus.Redis() {
		t.Fatal("a bus built with no client reported Redis")
	}
	bus.Invalidate("grants", memo.Key("tenant-a", ""))
	if _, ok := entries.Get(memo.Key("tenant-a", "user-1")); ok {
		t.Error("a local bus did not drop the entry")
	}
}

// Invalidating a cache nobody registered must not panic — a module can publish
// for a cache another build does not have.
func TestInvalidatingAnUnknownCacheIsHarmless(t *testing.T) {
	cache.NewBus(context.Background(), nil).Invalidate("nothing-registered", "x")
}

func redisBus(t *testing.T, ctx context.Context) *cache.Bus {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL is not set")
	}
	client := cache.Dial(url)
	if client == nil {
		t.Fatal("Dial returned no client for a configured URL")
	}
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis is not answering: %v", err)
	}
	bus := cache.NewBus(ctx, client)
	t.Cleanup(func() { _ = client.Close() })
	return bus
}

// The reason Redis is here at all: an administrator revokes a permission on one
// replica, and the replica serving somebody else's request has to stop honouring
// it without waiting for a timer.
func TestAnInvalidationOnOneReplicaReachesAnother(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	first := redisBus(t, ctx)
	second := redisBus(t, ctx)

	held := memo.New[string](time.Minute)
	held.Put(memo.Key("tenant-a", "user-1"), "granted")
	held.Put(memo.Key("tenant-b", "user-1"), "granted")
	second.Register("grants", held)

	// Subscriptions are established asynchronously; publishing into one that has
	// not connected yet would test nothing.
	time.Sleep(300 * time.Millisecond)

	first.Invalidate("grants", memo.Key("tenant-a", ""))

	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, ok := held.Get(memo.Key("tenant-a", "user-1")); !ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the other replica never dropped the entry")
		}
		time.Sleep(20 * time.Millisecond)
	}

	if _, ok := held.Get(memo.Key("tenant-b", "user-1")); !ok {
		t.Error("the other tenant's entry was dropped too")
	}
}
