package exchange

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const sampleKlinesJSON = `[
    [1700000000000, "10.0", "11.0", "9.0", "10.5", "100.0", 1700003600000, "1050", 5, "50", "525", "0"],
    [1700003600000, "10.5", "11.5", "10.0", "11.2", "150.0", 1700007200000, "1680", 8, "75", "840", "0"]
]`

func TestBinance_Klines_Success(t *testing.T) {
	var gotSymbol, gotInterval, gotLimit string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSymbol = r.URL.Query().Get("symbol")
		gotInterval = r.URL.Query().Get("interval")
		gotLimit = r.URL.Query().Get("limit")
		_, _ = w.Write([]byte(sampleKlinesJSON))
	}))
	defer server.Close()

	b := NewBinance(server.URL, 5*time.Second)
	candles, err := b.Klines(context.Background(), "BTCUSDT", "1h", 50)
	if err != nil {
		t.Fatalf("Klines: %v", err)
	}

	if gotSymbol != "BTCUSDT" {
		t.Errorf("symbol = %q", gotSymbol)
	}
	if gotInterval != "1h" {
		t.Errorf("interval = %q", gotInterval)
	}
	if gotLimit != "50" {
		t.Errorf("limit = %q", gotLimit)
	}

	if len(candles) != 2 {
		t.Fatalf("got %d candles, want 2", len(candles))
	}
	if candles[0].Close != 10.5 {
		t.Errorf("first close = %v, want 10.5", candles[0].Close)
	}
	if candles[1].High != 11.5 {
		t.Errorf("second high = %v, want 11.5", candles[1].High)
	}
	if candles[0].OpenTime.UnixMilli() != 1700000000000 {
		t.Errorf("first open time mismatch: %v", candles[0].OpenTime.UnixMilli())
	}
}

func TestBinance_Klines_RejectsEmptyArgs(t *testing.T) {
	b := NewBinance("http://localhost", time.Second)

	cases := []struct {
		symbol, interval string
	}{
		{"", "1h"},
		{"BTCUSDT", ""},
		{"", ""},
	}
	for _, c := range cases {
		if _, err := b.Klines(context.Background(), c.symbol, c.interval, 100); err == nil {
			t.Errorf("expected error for symbol=%q interval=%q", c.symbol, c.interval)
		}
	}
}

func TestBinance_Klines_LimitClamping(t *testing.T) {
	cases := []struct {
		input    int
		wantSent string
	}{
		{0, "100"},
		{-5, "100"},
		{50, "50"},
		{1000, "1000"},
		{5000, "1000"},
	}
	for _, c := range cases {
		var sent string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sent = r.URL.Query().Get("limit")
			_, _ = w.Write([]byte("[]"))
		}))
		b := NewBinance(server.URL, time.Second)
		if _, err := b.Klines(context.Background(), "BTCUSDT", "1h", c.input); err != nil {
			t.Fatalf("Klines(%d): %v", c.input, err)
		}
		if sent != c.wantSent {
			t.Errorf("input=%d sent=%q, want %q", c.input, sent, c.wantSent)
		}
		server.Close()
	}
}

func TestBinance_Klines_NonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limit", http.StatusTooManyRequests)
	}))
	defer server.Close()

	b := NewBinance(server.URL, 5*time.Second)
	_, err := b.Klines(context.Background(), "BTCUSDT", "1h", 100)
	if err == nil {
		t.Fatal("expected error for 429")
	}
	if !strings.Contains(err.Error(), "status 429") {
		t.Errorf("error: %v", err)
	}
}

func TestBinance_Klines_MalformedRow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[[1,2,3]]`))
	}))
	defer server.Close()

	b := NewBinance(server.URL, 5*time.Second)
	_, err := b.Klines(context.Background(), "BTCUSDT", "1h", 100)
	if err == nil {
		t.Fatal("expected error for short row")
	}
}

func TestBinance_Klines_BadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	b := NewBinance(server.URL, 5*time.Second)
	_, err := b.Klines(context.Background(), "BTCUSDT", "1h", 100)
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestBinance_Klines_RespectsContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	b := NewBinance(server.URL, 5*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.Klines(ctx, "BTCUSDT", "1h", 100); err == nil {
		t.Fatal("expected error from canceled context")
	}
}
