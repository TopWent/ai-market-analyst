// Package api wires the market-data HTTP handlers.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TopWent/ai-market-analyst/go-service/internal/exchange"
	"github.com/TopWent/ai-market-analyst/go-service/internal/indicators"
)

type SnapshotProvider interface {
	Klines(ctx context.Context, symbol, interval string, limit int) ([]exchange.Candle, error)
}

type Handler struct {
	Provider SnapshotProvider
	Logger   *slog.Logger
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /snapshot", h.Snapshot)
	return mux
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Snapshot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	symbol := strings.ToUpper(r.URL.Query().Get("symbol"))
	interval := r.URL.Query().Get("interval")
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer")
			return
		}
		limit = n
	}

	if symbol == "" || interval == "" {
		writeError(w, http.StatusBadRequest, "symbol and interval are required")
		return
	}

	candles, err := h.Provider.Klines(ctx, symbol, interval, limit)
	if err != nil {
		h.logger().Warn("klines failed",
			"symbol", symbol, "interval", interval, "err", err)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("upstream: %s", err))
		return
	}
	if len(candles) == 0 {
		writeError(w, http.StatusNotFound, "no candles returned")
		return
	}

	closes := make([]float64, len(candles))
	for i, c := range candles {
		closes[i] = c.Close
	}

	macd := indicators.MACD(closes, 12, 26, 9)
	boll := indicators.Bollinger(closes, 20, 2)

	resp := SnapshotResponse{
		Symbol:    symbol,
		Interval:  interval,
		FetchedAt: time.Now().UTC(),
		Candles:   toCandleResponses(candles),
		Indicators: IndicatorsResponse{
			EMA12: closes2series(indicators.EMA(closes, 12)),
			EMA26: closes2series(indicators.EMA(closes, 26)),
			RSI14: closes2series(indicators.RSI(closes, 14)),
			MACD: MACDResponse{
				MACD:      closes2series(macd.MACD),
				Signal:    closes2series(macd.Signal),
				Histogram: closes2series(macd.Histogram),
			},
			Bollinger: BollingerResponse{
				Upper:  closes2series(boll.Upper),
				Middle: closes2series(boll.Middle),
				Lower:  closes2series(boll.Lower),
			},
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) logger() *slog.Logger {
	if h.Logger == nil {
		return slog.Default()
	}
	return h.Logger
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type SnapshotResponse struct {
	Symbol     string             `json:"symbol"`
	Interval   string             `json:"interval"`
	FetchedAt  time.Time          `json:"fetched_at"`
	Candles    []CandleResponse   `json:"candles"`
	Indicators IndicatorsResponse `json:"indicators"`
}

type CandleResponse struct {
	OpenTime time.Time `json:"open_time"`
	Open     float64   `json:"open"`
	High     float64   `json:"high"`
	Low      float64   `json:"low"`
	Close    float64   `json:"close"`
	Volume   float64   `json:"volume"`
}

type IndicatorsResponse struct {
	EMA12     FloatSeries       `json:"ema_12"`
	EMA26     FloatSeries       `json:"ema_26"`
	RSI14     FloatSeries       `json:"rsi_14"`
	MACD      MACDResponse      `json:"macd"`
	Bollinger BollingerResponse `json:"bollinger"`
}

type MACDResponse struct {
	MACD      FloatSeries `json:"macd"`
	Signal    FloatSeries `json:"signal"`
	Histogram FloatSeries `json:"histogram"`
}

type BollingerResponse struct {
	Upper  FloatSeries `json:"upper"`
	Middle FloatSeries `json:"middle"`
	Lower  FloatSeries `json:"lower"`
}

// FloatSeries renders NaN and Inf as JSON null.
type FloatSeries []float64

func (s FloatSeries) MarshalJSON() ([]byte, error) {
	if len(s) == 0 {
		return []byte("[]"), nil
	}
	var b strings.Builder
	b.Grow(len(s) * 8)
	b.WriteByte('[')
	for i, v := range s {
		if i > 0 {
			b.WriteByte(',')
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			b.WriteString("null")
		} else {
			b.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
		}
	}
	b.WriteByte(']')
	return []byte(b.String()), nil
}

func closes2series(values []float64) FloatSeries {
	return FloatSeries(values)
}

func toCandleResponses(in []exchange.Candle) []CandleResponse {
	out := make([]CandleResponse, len(in))
	for i, c := range in {
		out[i] = CandleResponse{
			OpenTime: c.OpenTime,
			Open:     c.Open,
			High:     c.High,
			Low:      c.Low,
			Close:    c.Close,
			Volume:   c.Volume,
		}
	}
	return out
}
