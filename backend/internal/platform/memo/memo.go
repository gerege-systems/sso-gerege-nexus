// Package memo is a small time-bounded cache for answers the database gives
// again and again within the life of one screen.
//
// It exists for the two lookups that sit in front of nearly every authenticated
// request — which apps a tenant has installed, and what a member is allowed to
// do — and it is deliberately not a general-purpose cache. Both answers are
// authorisation decisions, so two properties matter more than hit rate: an
// entry expires on its own, and a change made through this process drops the
// entries it affects immediately rather than waiting for that expiry.
//
// Across replicas only the first property holds. A permission revoked on one
// API instance is still honoured by another until its copy expires, which is
// what keeps the window short rather than convenient.
package memo

import (
	"strings"
	"sync"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/async"
)

type entry[V any] struct {
	value   V
	expires time.Time
}

// Cache maps a string key to a value that stops being trusted after a while.
// The zero value is not usable; call New.
type Cache[V any] struct {
	ttl     time.Duration
	mu      sync.RWMutex
	entries map[string]entry[V]
}

// New builds a cache whose entries live for ttl and starts the sweep that keeps
// it from growing with every key ever asked for.
func New[V any](ttl time.Duration) *Cache[V] {
	c := &Cache[V]{ttl: ttl, entries: make(map[string]entry[V])}
	async.Go("memo-sweep", c.sweepForever)
	return c
}

// Get returns a live entry. An expired one is reported as absent and left for
// the sweep, so a read never takes the write lock.
func (c *Cache[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	found, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(found.expires) {
		var zero V
		return zero, false
	}
	return found.value, true
}

// Put stores a value for the configured lifetime.
func (c *Cache[V]) Put(key string, value V) {
	c.mu.Lock()
	c.entries[key] = entry[V]{value: value, expires: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

// InvalidatePrefix drops every entry whose key starts with prefix. Both callers
// key by tenant first, so this is how one tenant's administrator changing
// something stops affecting only their own next request.
func (c *Cache[V]) InvalidatePrefix(prefix string) {
	c.mu.Lock()
	for key := range c.entries {
		if strings.HasPrefix(key, prefix) {
			delete(c.entries, key)
		}
	}
	c.mu.Unlock()
}

// Len reports how many entries are held, live or not. Tests use it; nothing
// else should need it.
func (c *Cache[V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *Cache[V]) sweepForever() {
	// Twice the lifetime: an entry is unreadable the moment it expires, so
	// sweeping is about memory, not correctness, and does not need to be prompt.
	ticker := time.NewTicker(2 * c.ttl)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for key, held := range c.entries {
			if now.After(held.expires) {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}

// Key joins the parts of a composite key with a separator that cannot appear in
// a UUID or an app id, so tenant "a" + app "b" can never collide with tenant
// "a\x00b" + no app.
func Key(parts ...string) string {
	return strings.Join(parts, "\x00")
}
