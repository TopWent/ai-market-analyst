// Package indicators implements EMA, RSI, MACD, and Bollinger Bands.
package indicators

import "math"

// EMA returns the exponential moving average over period.
func EMA(prices []float64, period int) []float64 {
	n := len(prices)
	out := makeNaN(n)
	if period <= 0 || n < period {
		return out
	}

	var sum float64
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	out[period-1] = sum / float64(period)

	alpha := 2.0 / float64(period+1)
	for i := period; i < n; i++ {
		out[i] = prices[i]*alpha + out[i-1]*(1-alpha)
	}
	return out
}

// RSI returns Wilder's RSI over period.
func RSI(prices []float64, period int) []float64 {
	n := len(prices)
	out := makeNaN(n)
	if period <= 0 || n < period+1 {
		return out
	}

	var gain, loss float64
	for i := 1; i <= period; i++ {
		delta := prices[i] - prices[i-1]
		if delta > 0 {
			gain += delta
		} else {
			loss -= delta
		}
	}
	gain /= float64(period)
	loss /= float64(period)
	out[period] = computeRSI(gain, loss)

	for i := period + 1; i < n; i++ {
		delta := prices[i] - prices[i-1]
		var g, l float64
		if delta > 0 {
			g = delta
		} else {
			l = -delta
		}
		gain = (gain*float64(period-1) + g) / float64(period)
		loss = (loss*float64(period-1) + l) / float64(period)
		out[i] = computeRSI(gain, loss)
	}
	return out
}

func computeRSI(gain, loss float64) float64 {
	if loss == 0 {
		if gain == 0 {
			return 50
		}
		return 100
	}
	rs := gain / loss
	return 100 - 100/(1+rs)
}

type MACDResult struct {
	MACD      []float64
	Signal    []float64
	Histogram []float64
}

// MACD returns MACD line (EMA(fast) - EMA(slow)), signal line and histogram.
func MACD(prices []float64, fast, slow, signal int) MACDResult {
	n := len(prices)
	macdLine := makeNaN(n)

	if fast <= 0 || slow <= 0 || signal <= 0 || fast >= slow {
		return MACDResult{MACD: macdLine, Signal: makeNaN(n), Histogram: makeNaN(n)}
	}

	fastEMA := EMA(prices, fast)
	slowEMA := EMA(prices, slow)

	for i := 0; i < n; i++ {
		if !math.IsNaN(fastEMA[i]) && !math.IsNaN(slowEMA[i]) {
			macdLine[i] = fastEMA[i] - slowEMA[i]
		}
	}

	signalLine := emaWithLeadingNaN(macdLine, signal)

	histogram := makeNaN(n)
	for i := 0; i < n; i++ {
		if !math.IsNaN(macdLine[i]) && !math.IsNaN(signalLine[i]) {
			histogram[i] = macdLine[i] - signalLine[i]
		}
	}

	return MACDResult{MACD: macdLine, Signal: signalLine, Histogram: histogram}
}

func emaWithLeadingNaN(values []float64, period int) []float64 {
	n := len(values)
	out := makeNaN(n)

	start := 0
	for start < n && math.IsNaN(values[start]) {
		start++
	}
	if period <= 0 || n-start < period {
		return out
	}

	var sum float64
	for i := 0; i < period; i++ {
		sum += values[start+i]
	}
	out[start+period-1] = sum / float64(period)

	alpha := 2.0 / float64(period+1)
	for i := start + period; i < n; i++ {
		out[i] = values[i]*alpha + out[i-1]*(1-alpha)
	}
	return out
}

type BollingerResult struct {
	Upper  []float64
	Middle []float64
	Lower  []float64
}

// Bollinger returns Bollinger Bands with the given period and stddev multiplier.
func Bollinger(prices []float64, period int, stdDev float64) BollingerResult {
	n := len(prices)
	res := BollingerResult{
		Upper:  makeNaN(n),
		Middle: makeNaN(n),
		Lower:  makeNaN(n),
	}
	if period <= 0 || n < period {
		return res
	}

	for i := period - 1; i < n; i++ {
		var sum float64
		for j := i - period + 1; j <= i; j++ {
			sum += prices[j]
		}
		mean := sum / float64(period)

		var sq float64
		for j := i - period + 1; j <= i; j++ {
			d := prices[j] - mean
			sq += d * d
		}
		sd := math.Sqrt(sq / float64(period))

		res.Middle[i] = mean
		res.Upper[i] = mean + stdDev*sd
		res.Lower[i] = mean - stdDev*sd
	}
	return res
}

func makeNaN(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	return out
}
