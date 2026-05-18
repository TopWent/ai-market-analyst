package exchange

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// defaultMaxEntries caps the number of cached keys so the map can't grow without
// bound when callers hit many distinct symbol/interval/limit combinations.
const defaultMaxEntries = 1024

type cachedEntry struct {
	candles []Candle
	expiry  time.Time
}

// CachedFetcher wraps another fetcher with a per-(symbol, interval, limit) TTL
// cache. Concurrent misses for the same key are collapsed into a single upstream
// call via singleflight, and the key set is bounded by maxEntries.
type CachedFetcher struct {
	inner      Fetcher
	ttl        time.Duration
	maxEntries int
	now        func() time.Time
	group      singleflight.Group

	mu      sync.RWMutex
	entries map[string]cachedEntry
}

// Fetcher abstracts the upstream so the cache can wrap any candle source.
type Fetcher interface {
	Klines(ctx context.Context, symbol, interval string, limit int) ([]Candle, error)
}

func NewCachedFetcher(inner Fetcher, ttl time.Duration) *CachedFetcher {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &CachedFetcher{
		inner:      inner,
		ttl:        ttl,
		maxEntries: defaultMaxEntries,
		now:        time.Now,
		entries:    make(map[string]cachedEntry),
	}
}

func (c *CachedFetcher) Klines(ctx context.Context, symbol, interval string, limit int) ([]Candle, error) {
	key := cacheKey(symbol, interval, limit)

	if candles, ok := c.lookup(key); ok {
		return candles, nil
	}

	// Only one goroutine per key fetches; the rest wait and share the result.
	v, err, _ := c.group.Do(key, func() (any, error) {
		// Another waiter may have populated the cache between our miss and the
		// singleflight entry, so check once more before going upstream.
		if candles, ok := c.lookup(key); ok {
			return candles, nil
		}

		candles, err := c.inner.Klines(ctx, symbol, interval, limit)
		if err != nil {
			return nil, err
		}
		c.store(key, candles)
		return candles, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]Candle), nil
}

func (c *CachedFetcher) lookup(key string) ([]Candle, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if ok && c.now().Before(entry.expiry) {
		return entry.candles, true
	}
	return nil, false
}

func (c *CachedFetcher) store(key string, candles []Candle) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.maxEntries {
		c.evictLocked()
	}
	c.entries[key] = cachedEntry{candles: candles, expiry: c.now().Add(c.ttl)}
}

// evictLocked makes room for a new key. It drops expired entries first; if none
// are expired it drops the entry closest to expiry. Caller must hold c.mu.
func (c *CachedFetcher) evictLocked() {
	now := c.now()
	var oldestKey string
	var oldestExpiry time.Time
	for k, e := range c.entries {
		if !now.Before(e.expiry) {
			delete(c.entries, k)
			return
		}
		if oldestKey == "" || e.expiry.Before(oldestExpiry) {
			oldestKey, oldestExpiry = k, e.expiry
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// Size returns the number of cached keys.
func (c *CachedFetcher) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func cacheKey(symbol, interval string, limit int) string {
	return fmt.Sprintf("%s|%s|%d", symbol, interval, limit)
}
