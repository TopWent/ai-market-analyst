package indicators

import (
	"math"
	"testing"
)

const epsilon = 1e-9

func nearly(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.IsNaN(a) && math.IsNaN(b)
	}
	return math.Abs(a-b) < epsilon
}

func TestEMA_LeadingNaN(t *testing.T) {
	prices := []float64{1, 2, 3, 4, 5}
	out := EMA(prices, 3)
	for i := 0; i < 2; i++ {
		if !math.IsNaN(out[i]) {
			t.Errorf("out[%d] = %v, want NaN", i, out[i])
		}
	}
}

func TestEMA_SeededWithSMA(t *testing.T) {
	prices := []float64{10, 20, 30, 40, 50}
	out := EMA(prices, 3)
	if !nearly(out[2], 20) {
		t.Errorf("seed = %v, want 20 (SMA of first 3)", out[2])
	}
}

func TestEMA_FollowsRecursion(t *testing.T) {
	prices := []float64{10, 20, 30, 40, 50}
	out := EMA(prices, 3)
	alpha := 2.0 / 4.0
	want3 := 40*alpha + out[2]*(1-alpha)
	want4 := 50*alpha + want3*(1-alpha)
	if !nearly(out[3], want3) {
		t.Errorf("out[3] = %v, want %v", out[3], want3)
	}
	if !nearly(out[4], want4) {
		t.Errorf("out[4] = %v, want %v", out[4], want4)
	}
}

func TestEMA_FlatPrices(t *testing.T) {
	prices := []float64{5, 5, 5, 5, 5, 5}
	out := EMA(prices, 3)
	for i := 2; i < len(out); i++ {
		if !nearly(out[i], 5) {
			t.Errorf("out[%d] = %v, want 5 (flat input)", i, out[i])
		}
	}
}

func TestEMA_InsufficientData(t *testing.T) {
	out := EMA([]float64{1, 2}, 5)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	for i, v := range out {
		if !math.IsNaN(v) {
			t.Errorf("out[%d] = %v, want NaN", i, v)
		}
	}
}

func TestEMA_ZeroOrNegativePeriod(t *testing.T) {
	out := EMA([]float64{1, 2, 3}, 0)
	for i, v := range out {
		if !math.IsNaN(v) {
			t.Errorf("out[%d] = %v, want NaN for period=0", i, v)
		}
	}
}

func TestRSI_StrictlyRisingIs100(t *testing.T) {
	prices := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	out := RSI(prices, 3)
	for i := 3; i < len(out); i++ {
		if !nearly(out[i], 100) {
			t.Errorf("out[%d] = %v, want 100 for strictly rising", i, out[i])
		}
	}
}

func TestRSI_StrictlyFallingIs0(t *testing.T) {
	prices := []float64{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	out := RSI(prices, 3)
	for i := 3; i < len(out); i++ {
		if !nearly(out[i], 0) {
			t.Errorf("out[%d] = %v, want 0 for strictly falling", i, out[i])
		}
	}
}

func TestRSI_FlatPricesIs50(t *testing.T) {
	prices := []float64{5, 5, 5, 5, 5, 5, 5, 5}
	out := RSI(prices, 3)
	for i := 3; i < len(out); i++ {
		if !nearly(out[i], 50) {
			t.Errorf("out[%d] = %v, want 50 for flat input", i, out[i])
		}
	}
}

func TestRSI_LeadingNaN(t *testing.T) {
	prices := []float64{1, 2, 3, 4, 5}
	out := RSI(prices, 3)
	for i := 0; i < 3; i++ {
		if !math.IsNaN(out[i]) {
			t.Errorf("out[%d] = %v, want NaN (leading window)", i, out[i])
		}
	}
}

func TestRSI_RangeIsBounded(t *testing.T) {
	prices := []float64{
		44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08,
		45.89, 46.03, 45.61, 46.28, 46.28, 46.00, 46.03, 46.41, 46.22, 45.64,
	}
	out := RSI(prices, 14)
	for i, v := range out {
		if math.IsNaN(v) {
			continue
		}
		if v < 0 || v > 100 {
			t.Errorf("out[%d] = %v, must be in [0, 100]", i, v)
		}
	}
}

func TestMACD_LengthMatchesInput(t *testing.T) {
	prices := make([]float64, 50)
	for i := range prices {
		prices[i] = float64(i)
	}
	res := MACD(prices, 12, 26, 9)
	if len(res.MACD) != len(prices) {
		t.Errorf("MACD len = %d, want %d", len(res.MACD), len(prices))
	}
	if len(res.Signal) != len(prices) {
		t.Errorf("Signal len = %d, want %d", len(res.Signal), len(prices))
	}
	if len(res.Histogram) != len(prices) {
		t.Errorf("Histogram len = %d, want %d", len(res.Histogram), len(prices))
	}
}

func TestMACD_HistogramEqualsMACDMinusSignal(t *testing.T) {
	prices := make([]float64, 60)
	for i := range prices {
		prices[i] = math.Sin(float64(i)/5) * 10
	}
	res := MACD(prices, 12, 26, 9)
	for i := range prices {
		if math.IsNaN(res.MACD[i]) || math.IsNaN(res.Signal[i]) {
			if !math.IsNaN(res.Histogram[i]) {
				t.Errorf("Histogram[%d] should be NaN when MACD or Signal is", i)
			}
			continue
		}
		want := res.MACD[i] - res.Signal[i]
		if !nearly(res.Histogram[i], want) {
			t.Errorf("Histogram[%d] = %v, want %v", i, res.Histogram[i], want)
		}
	}
}

func TestMACD_RejectsInvalidPeriods(t *testing.T) {
	prices := []float64{1, 2, 3, 4, 5}

	// Fast >= slow is invalid.
	res := MACD(prices, 26, 12, 9)
	for i, v := range res.MACD {
		if !math.IsNaN(v) {
			t.Errorf("MACD[%d] = %v, want NaN for fast>=slow", i, v)
		}
	}

	// Zero period.
	res = MACD(prices, 0, 26, 9)
	for i, v := range res.MACD {
		if !math.IsNaN(v) {
			t.Errorf("MACD[%d] = %v, want NaN for zero period", i, v)
		}
	}
}

func TestBollinger_MiddleIsSMA(t *testing.T) {
	prices := []float64{2, 4, 6, 8, 10}
	res := Bollinger(prices, 3, 2)
	want := (2.0 + 4.0 + 6.0) / 3
	if !nearly(res.Middle[2], want) {
		t.Errorf("Middle[2] = %v, want %v", res.Middle[2], want)
	}
	want = (4.0 + 6.0 + 8.0) / 3
	if !nearly(res.Middle[3], want) {
		t.Errorf("Middle[3] = %v, want %v", res.Middle[3], want)
	}
}

func TestBollinger_BandsSymmetricAroundMiddle(t *testing.T) {
	prices := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	res := Bollinger(prices, 5, 2)
	for i := 4; i < len(prices); i++ {
		halfUpper := res.Upper[i] - res.Middle[i]
		halfLower := res.Middle[i] - res.Lower[i]
		if !nearly(halfUpper, halfLower) {
			t.Errorf("bands not symmetric at %d: upper-mid=%v, mid-lower=%v",
				i, halfUpper, halfLower)
		}
	}
}

func TestBollinger_FlatPricesCollapseBands(t *testing.T) {
	prices := []float64{5, 5, 5, 5, 5, 5}
	res := Bollinger(prices, 3, 2)
	for i := 2; i < len(prices); i++ {
		if !nearly(res.Upper[i], 5) || !nearly(res.Middle[i], 5) || !nearly(res.Lower[i], 5) {
			t.Errorf("at i=%d expected all bands = 5, got upper=%v mid=%v lower=%v",
				i, res.Upper[i], res.Middle[i], res.Lower[i])
		}
	}
}

func TestBollinger_LeadingNaN(t *testing.T) {
	prices := []float64{1, 2, 3, 4, 5}
	res := Bollinger(prices, 3, 2)
	for i := 0; i < 2; i++ {
		if !math.IsNaN(res.Middle[i]) {
			t.Errorf("Middle[%d] = %v, want NaN", i, res.Middle[i])
		}
	}
}

func TestBollinger_InvalidPeriod(t *testing.T) {
	prices := []float64{1, 2, 3}
	res := Bollinger(prices, 0, 2)
	for i := range prices {
		if !math.IsNaN(res.Middle[i]) {
			t.Errorf("Middle[%d] should be NaN for period=0", i)
		}
	}
}
