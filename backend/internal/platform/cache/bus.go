// Package cache makes one process's cached authorisation answers stop being
// true everywhere at once.
//
// What is cached — a member's effective permissions, whether a tenant has an
// app installed, a session's claims — stays in the process that cached it. None
// of it is put in Redis. The lookups are single-digit milliseconds against a
// database that is already there, so a shared copy would trade one network hop
// for another; what an in-process cache cannot do on its own is find out that
// somebody else's replica has just revoked something.
//
// So Redis carries invalidation, not values. A replica that changes a role
// publishes which cache and which key prefix stopped being true, and every
// replica — including the one that published it — drops those entries. With one
// replica this is exactly the local invalidation it replaces. With several it is
// the difference between a revoked permission taking effect now and taking
// effect when a timer somewhere else runs out.
//
// Redis is optional. Without REDIS_URL the bus is local-only and the platform
// behaves as it did: correct on one replica, eventually correct on more.
package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/async"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// channel is the single Redis pub/sub channel every invalidation travels on.
// One channel rather than one per cache: the volume is an administrator
// pressing a button, and a subscriber per cache would be a connection per cache.
const channel = "gerege_nexus:invalidate"

// Invalidatable is the part of a cache the bus needs. memo.Cache satisfies it.
type Invalidatable interface {
	InvalidatePrefix(prefix string)
}

type message struct {
	// Origin is the instance that published. A replica ignores its own messages
	// because it has already applied them locally — waiting for the round trip
	// would mean the administrator who pressed the button is the one person who
	// still sees the old answer.
	Origin string `json:"origin"`
	Cache  string `json:"cache"`
	Prefix string `json:"prefix"`
}

// Bus fans invalidations out to every replica.
type Bus struct {
	origin string
	client *redis.Client

	mu     sync.RWMutex
	caches map[string]Invalidatable
}

// Dial builds the shared Redis client, or nil when none is configured.
//
// One client for the whole process: the bus subscribes on it and the shared
// rate limiters count on it, and go-redis pools connections underneath. A nil
// return is a supported configuration, not a failure — everything that takes a
// client accepts nil and falls back to what the platform did before.
func Dial(url string) *redis.Client {
	if url == "" {
		slog.Info("cache: REDIS_URL is not set; invalidation stays inside this process "+
			"and rate limits are counted per replica", "redis", false)
		return nil
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		slog.Error("cache: REDIS_URL could not be parsed; continuing without Redis", "error", err)
		return nil
	}
	return redis.NewClient(opts)
}

// NewBus builds the bus. A nil client gives a local-only bus, which is a
// supported configuration rather than a degraded one: a single-replica
// deployment needs nothing else.
//
// It does not fail when Redis is unreachable. The subscriber retries, and until
// it connects the platform is in the state it would be in with no Redis at all
// — which is the state it shipped in for its whole life so far. Refusing to
// start would turn a cache-coherence dependency into an availability one.
func NewBus(ctx context.Context, client *redis.Client) *Bus {
	b := &Bus{origin: uuid.NewString(), caches: make(map[string]Invalidatable)}
	if client == nil {
		return b
	}
	b.client = client
	async.Go("cache-invalidation-subscriber", func() { b.subscribe(ctx) })
	return b
}

// Register names a cache so invalidations can be addressed to it.
func (b *Bus) Register(name string, c Invalidatable) {
	b.mu.Lock()
	b.caches[name] = c
	b.mu.Unlock()
}

// Invalidate drops the entries here and tells everybody else to do the same.
//
// The local drop happens first and does not depend on Redis: a publish that
// fails must not leave this process serving what it has just been told is
// wrong. What is lost when Redis is down is the other replicas, and they still
// have their expiry.
func (b *Bus) Invalidate(name, prefix string) {
	b.applyLocally(name, prefix)
	if b.client == nil {
		return
	}
	payload, err := json.Marshal(message{Origin: b.origin, Cache: name, Prefix: prefix})
	if err != nil {
		return
	}
	// Publishing is not worth making a caller wait on, but it is worth a
	// deadline: an unreachable Redis should not hold a request open.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	async.Go("cache-invalidation-publish", func() {
		defer cancel()
		if err := b.client.Publish(ctx, channel, payload).Err(); err != nil {
			slog.Warn("cache: could not publish an invalidation; other replicas will wait for expiry",
				"cache", name, "error", err)
		}
	})
}

// Redis reports whether invalidations reach other replicas.
func (b *Bus) Redis() bool { return b.client != nil }

// Client exposes the shared connection so the rate limiters can count on it.
// It is nil when no Redis is configured.
func (b *Bus) Client() *redis.Client { return b.client }

func (b *Bus) applyLocally(name, prefix string) {
	b.mu.RLock()
	target := b.caches[name]
	b.mu.RUnlock()
	if target != nil {
		target.InvalidatePrefix(prefix)
	}
}

func (b *Bus) subscribe(ctx context.Context) {
	sub := b.client.Subscribe(ctx, channel)
	defer func() { _ = sub.Close() }()

	// Channel() reconnects on its own, so a Redis restart costs the messages
	// published while it was down and nothing else. Those entries still expire.
	for msg := range sub.Channel() {
		var received message
		if err := json.Unmarshal([]byte(msg.Payload), &received); err != nil {
			continue
		}
		if received.Origin == b.origin {
			continue
		}
		b.applyLocally(received.Cache, received.Prefix)
	}
	// Shutdown closes the channel too, and a warning on every clean stop trains
	// people to ignore the one that means something.
	if ctx.Err() != nil {
		return
	}
	slog.Warn("cache: the invalidation subscriber stopped; this replica is on expiry alone")
}
