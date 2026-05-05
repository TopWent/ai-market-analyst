package exchange

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type countingFetcher struct {
	calls   atomic.Int32
	result  []Candle
	err     error
	delay   time.Duration
	lastKey string
}

func (f *countingFetcher) Klines(_ context.Context, symbol, interval string, limit int) ([]Candle, error) {
	f.calls.Add(1)
	f.lastKey = cacheKey(symbol, interval, limit)
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return f.result, f.err
}

func sampleCandles(n int) []Candle {
	out := make([]Candle, n)
	base := time.Unix(1_700_000_000, 0).UTC()
	for i := range out {
		out[i] = Candle{
			OpenTime: base.Add(time.Duration(i) * time.Hour),
			Open:     float64(i),
			Close:    float64(i) + 0.1,
		}
	}
	return out
}

func TestCachedFetcher_FirstCallHitsUpstream(t *testing.T) {
	inner := &countingFetcher{result: sampleCandles(3)}
	c := NewCachedFetcher(inner, time.Minute)

	got, err := c.Klines(context.Background(), "BTCUSDT", "1h", 100)
	if err != nil {
		t.Fatalf("Klines: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d candles", len(got))
	}
	if inner.calls.Load() != 1 {
		t.Errorf("upstream calls = %d, want 1", inner.calls.Load())
	}
}

func TestCachedFetcher_SecondCallServesFromCache(t *testing.T) {
	inner := &countingFetcher{result: sampleCandles(3)}
	c := NewCachedFetcher(inner, time.Minute)

	for i := 0; i < 5; i++ {
		if _, err := c.Klines(context.Background(), "BTCUSDT", "1h", 100); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if inner.calls.Load() != 1 {
		t.Errorf("upstream calls = %d, want 1 (cache should serve the rest)", inner.calls.Load())
	}
	if c.Size() != 1 {
		t.Errorf("cache size = %d, want 1", c.Size())
	}
}

func TestCachedFetcher_DistinctKeysAreSeparate(t *testing.T) {
	inner := &countingFetcher{result: sampleCandles(3)}
	c := NewCachedFetcher(inner, time.Minute)

	_, _ = c.Klines(context.Background(), "BTCUSDT", "1h", 100)
	_, _ = c.Klines(context.Background(), "ETHUSDT", "1h", 100)
	_, _ = c.Klines(context.Background(), "BTCUSDT", "4h", 100)
	_, _ = c.Klines(context.Background(), "BTCUSDT", "1h", 200)

	if inner.calls.Load() != 4 {
		t.Errorf("upstream calls = %d, want 4 (distinct keys)", inner.calls.Load())
	}
	if c.Size() != 4 {
		t.Errorf("cache size = %d, want 4", c.Size())
	}
}

func TestCachedFetcher_ExpiryTriggersRefetch(t *testing.T) {
	inner := &countingFetcher{result: sampleCandles(3)}
	c := NewCachedFetcher(inner, time.Minute)

	current := time.Now()
	c.now = func() time.Time { return current }

	_, _ = c.Klines(context.Background(), "BTCUSDT", "1h", 100)
	_, _ = c.Klines(context.Background(), "BTCUSDT", "1h", 100)
	if inner.calls.Load() != 1 {
		t.Fatalf("upstream calls before expiry = %d, want 1", inner.calls.Load())
	}

	current = current.Add(2 * time.Minute)
	_, _ = c.Klines(context.Background(), "BTCUSDT", "1h", 100)
	if inner.calls.Load() != 2 {
		t.Errorf("upstream calls after expiry = %d, want 2", inner.calls.Load())
	}
}

func TestCachedFetcher_UpstreamErrorNotCached(t *testing.T) {
	inner := &countingFetcher{err: errors.New("upstream gone")}
	c := NewCachedFetcher(inner, time.Minute)

	if _, err := c.Klines(context.Background(), "BTCUSDT", "1h", 100); err == nil {
		t.Fatal("expected error")
	}
	if _, err := c.Klines(context.Background(), "BTCUSDT", "1h", 100); err == nil {
		t.Fatal("expected error on retry")
	}
	if inner.calls.Load() != 2 {
		t.Errorf("upstream calls = %d, want 2 (errors must not poison cache)", inner.calls.Load())
	}
}

func TestCachedFetcher_NegativeTTLFallsBackToDefault(t *testing.T) {
	inner := &countingFetcher{result: sampleCandles(1)}
	c := NewCachedFetcher(inner, 0)

	if c.ttl != 30*time.Second {
		t.Errorf("ttl = %s, want 30s default", c.ttl)
	}
}
