package exchange

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type cachedEntry struct {
	candles []Candle
	expiry  time.Time
}

// CachedFetcher wraps another fetcher with a per-(symbol, interval, limit) TTL
// cache. Hot keys with the same parameters skip the upstream call until expiry.
type CachedFetcher struct {
	inner Fetcher
	ttl   time.Duration
	now   func() time.Time

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
		inner:   inner,
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]cachedEntry),
	}
}

func (c *CachedFetcher) Klines(ctx context.Context, symbol, interval string, limit int) ([]Candle, error) {
	key := cacheKey(symbol, interval, limit)

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if ok && c.now().Before(entry.expiry) {
		return entry.candles, nil
	}

	candles, err := c.inner.Klines(ctx, symbol, interval, limit)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[key] = cachedEntry{candles: candles, expiry: c.now().Add(c.ttl)}
	c.mu.Unlock()
	return candles, nil
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
