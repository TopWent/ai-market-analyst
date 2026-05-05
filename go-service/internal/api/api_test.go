package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TopWent/ai-market-analyst/go-service/internal/exchange"
)

type fakeProvider struct {
	candles []exchange.Candle
	err     error

	gotSymbol   string
	gotInterval string
	gotLimit    int
}

func (f *fakeProvider) Klines(_ context.Context, symbol, interval string, limit int) ([]exchange.Candle, error) {
	f.gotSymbol = symbol
	f.gotInterval = interval
	f.gotLimit = limit
	return f.candles, f.err
}

func newHandler(p SnapshotProvider) http.Handler {
	h := &Handler{
		Provider: p,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return h.Routes()
}

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(newHandler(&fakeProvider{}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestSnapshot_RequiresSymbolAndInterval(t *testing.T) {
	srv := httptest.NewServer(newHandler(&fakeProvider{}))
	defer srv.Close()

	cases := []string{
		"/snapshot",
		"/snapshot?symbol=BTCUSDT",
		"/snapshot?interval=1h",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			resp, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("%s status = %d, want 400", path, resp.StatusCode)
			}
		})
	}
}

func TestSnapshot_RejectsBadLimit(t *testing.T) {
	srv := httptest.NewServer(newHandler(&fakeProvider{}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/snapshot?symbol=BTC&interval=1h&limit=abc")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSnapshot_UpperCasesSymbol(t *testing.T) {
	fp := &fakeProvider{candles: makeCandles(40)}
	srv := httptest.NewServer(newHandler(fp))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/snapshot?symbol=btcusdt&interval=1h")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if fp.gotSymbol != "BTCUSDT" {
		t.Errorf("provider got symbol %q, want BTCUSDT", fp.gotSymbol)
	}
}

func TestSnapshot_UpstreamError(t *testing.T) {
	fp := &fakeProvider{err: errors.New("connection refused")}
	srv := httptest.NewServer(newHandler(fp))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/snapshot?symbol=BTC&interval=1h")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}

func TestSnapshot_NoCandles(t *testing.T) {
	srv := httptest.NewServer(newHandler(&fakeProvider{candles: []exchange.Candle{}}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/snapshot?symbol=BTC&interval=1h")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSnapshot_HappyPath(t *testing.T) {
	fp := &fakeProvider{candles: makeCandles(40)}
	srv := httptest.NewServer(newHandler(fp))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/snapshot?symbol=BTCUSDT&interval=1h&limit=50")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if fp.gotLimit != 50 {
		t.Errorf("provider got limit %d, want 50", fp.gotLimit)
	}

	var got SnapshotResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Symbol != "BTCUSDT" || got.Interval != "1h" {
		t.Errorf("response identity: %+v", got)
	}
	if len(got.Candles) != 40 {
		t.Errorf("candles = %d, want 40", len(got.Candles))
	}
	if len(got.Indicators.EMA12) != 40 {
		t.Errorf("EMA12 len = %d, want 40", len(got.Indicators.EMA12))
	}
}

func TestFloatSeries_MarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		in   FloatSeries
		want string
	}{
		{"empty", FloatSeries{}, "[]"},
		{"normal", FloatSeries{1, 2.5, 3}, "[1,2.5,3]"},
		{"NaN becomes null", FloatSeries{math.NaN(), 1.5}, "[null,1.5]"},
		{"Inf becomes null", FloatSeries{math.Inf(1), math.Inf(-1)}, "[null,null]"},
		{"all NaN", FloatSeries{math.NaN(), math.NaN()}, "[null,null]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestSnapshotResponse_NaNRoundTrip(t *testing.T) {
	resp := SnapshotResponse{
		Symbol:    "X",
		Interval:  "1h",
		FetchedAt: time.Unix(0, 0).UTC(),
		Indicators: IndicatorsResponse{
			EMA12: FloatSeries{math.NaN(), 1.5, math.NaN()},
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"ema_12":[null,1.5,null]`) {
		t.Errorf("NaN not rendered as null: %s", b)
	}
}

func makeCandles(n int) []exchange.Candle {
	out := make([]exchange.Candle, n)
	for i := range out {
		v := float64(i + 1)
		out[i] = exchange.Candle{
			OpenTime: time.Unix(int64(i)*3600, 0).UTC(),
			Open:     v,
			High:     v + 0.5,
			Low:      v - 0.5,
			Close:    v + 0.1,
			Volume:   v * 10,
		}
	}
	return out
}
