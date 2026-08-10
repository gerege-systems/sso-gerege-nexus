package memo_test

import (
	"testing"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/memo"
)

func TestAnEntryStopsBeingReadableWhenItExpires(t *testing.T) {
	cache := memo.New[int](40 * time.Millisecond)
	cache.Put("k", 7)

	if got, ok := cache.Get("k"); !ok || got != 7 {
		t.Fatalf("fresh entry: got %v, %v", got, ok)
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok := cache.Get("k"); ok {
		t.Error("an expired entry was still served")
	}
}

// The invalidation both callers rely on is by tenant, and both build their keys
// tenant-first. A prefix that matched loosely would drop a different tenant's
// entries, which is only a performance bug — but one that matched too tightly
// would leave a revoked permission in place, which is not.
func TestInvalidatePrefixDropsOneTenantAndLeavesTheOthers(t *testing.T) {
	cache := memo.New[string](time.Minute)
	cache.Put(memo.Key("tenant-a", "user-1"), "a1")
	cache.Put(memo.Key("tenant-a", "user-2"), "a2")
	cache.Put(memo.Key("tenant-b", "user-1"), "b1")
	// A tenant id that has the first one as a literal prefix: the separator is
	// what has to stop this from being caught.
	cache.Put(memo.Key("tenant-agency", "user-1"), "ag1")

	cache.InvalidatePrefix(memo.Key("tenant-a", ""))

	for _, key := range []string{memo.Key("tenant-a", "user-1"), memo.Key("tenant-a", "user-2")} {
		if _, ok := cache.Get(key); ok {
			t.Errorf("%q survived its tenant being invalidated", key)
		}
	}
	for _, key := range []string{memo.Key("tenant-b", "user-1"), memo.Key("tenant-agency", "user-1")} {
		if _, ok := cache.Get(key); !ok {
			t.Errorf("%q was dropped with another tenant's entries", key)
		}
	}
}

// Two tenants whose ids differ only in where a separator would fall must not
// share an entry — that would be one tenant reading another's permissions.
func TestKeyPartsCannotRunTogether(t *testing.T) {
	if memo.Key("a", "bc") == memo.Key("ab", "c") {
		t.Fatal("two different key part splits produced the same key")
	}
}

func TestConcurrentUseIsSafe(t *testing.T) {
	cache := memo.New[int](time.Minute)
	done := make(chan struct{})
	for worker := range 8 {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := range 200 {
				key := memo.Key("tenant", string(rune('a'+worker)))
				cache.Put(key, i)
				cache.Get(key)
				if i%50 == 0 {
					cache.InvalidatePrefix(memo.Key("tenant", ""))
				}
			}
		}()
	}
	for range 8 {
		<-done
	}
}
