// Package exchange fetches OHLCV candles from public exchange REST APIs.
package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Candle struct {
	OpenTime time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
}

type Binance struct {
	BaseURL string
	Client  *http.Client
}

func NewBinance(baseURL string, timeout time.Duration) *Binance {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Binance{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: timeout},
	}
}

// Klines returns up to limit OHLCV candles for symbol at interval.
// limit is clamped to [1, 1000]; the Binance default is 500.
func (b *Binance) Klines(ctx context.Context, symbol, interval string, limit int) ([]Candle, error) {
	if symbol == "" || interval == "" {
		return nil, fmt.Errorf("symbol and interval required")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	u, err := url.Parse(b.BaseURL + "/api/v3/klines")
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	q := u.Query()
	q.Set("symbol", symbol)
	q.Set("interval", interval)
	q.Set("limit", strconv.Itoa(limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	resp, err := b.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var raw [][]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	out := make([]Candle, 0, len(raw))
	for i, row := range raw {
		if len(row) < 6 {
			return nil, fmt.Errorf("row %d malformed", i)
		}
		c, err := parseCandle(row)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i, err)
		}
		out = append(out, c)
	}
	return out, nil
}

func parseCandle(row []any) (Candle, error) {
	var c Candle
	openMs, ok := row[0].(float64)
	if !ok {
		return c, fmt.Errorf("open time not a number")
	}
	c.OpenTime = time.UnixMilli(int64(openMs))

	var err error
	if c.Open, err = parseStringFloat(row[1]); err != nil {
		return c, fmt.Errorf("open: %w", err)
	}
	if c.High, err = parseStringFloat(row[2]); err != nil {
		return c, fmt.Errorf("high: %w", err)
	}
	if c.Low, err = parseStringFloat(row[3]); err != nil {
		return c, fmt.Errorf("low: %w", err)
	}
	if c.Close, err = parseStringFloat(row[4]); err != nil {
		return c, fmt.Errorf("close: %w", err)
	}
	if c.Volume, err = parseStringFloat(row[5]); err != nil {
		return c, fmt.Errorf("volume: %w", err)
	}
	return c, nil
}

func parseStringFloat(v any) (float64, error) {
	s, ok := v.(string)
	if !ok {
		return 0, fmt.Errorf("expected string, got %T", v)
	}
	return strconv.ParseFloat(s, 64)
}
