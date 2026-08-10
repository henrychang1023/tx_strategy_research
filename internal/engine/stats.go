package engine

import "math"

// tradingDaysPerYear is the standard annualization factor.
const tradingDaysPerYear = 252

// sharpe computes an annualized, zero-risk-free-rate Sharpe ratio from a
// series of daily returns (points or fractional returns; the caller decides
// the unit). Returns 0 for fewer than 2 observations or zero variance.
func sharpe(dailyReturns []float64) float64 {
	n := len(dailyReturns)
	if n < 2 {
		return 0
	}
	var sum float64
	for _, r := range dailyReturns {
		sum += r
	}
	mean := sum / float64(n)

	var sumSq float64
	for _, r := range dailyReturns {
		d := r - mean
		sumSq += d * d
	}
	stdev := math.Sqrt(sumSq / float64(n-1))
	if stdev == 0 {
		return 0
	}
	return mean / stdev * math.Sqrt(tradingDaysPerYear)
}

// maxDrawdown returns the largest peak-to-trough decline in cumulative (a
// running series, e.g. cumulative points or cumulative return), as a
// positive magnitude in the same unit as cumulative.
func maxDrawdown(cumulative []float64) float64 {
	if len(cumulative) == 0 {
		return 0
	}
	peak := cumulative[0]
	var maxDD float64
	for _, v := range cumulative {
		if v > peak {
			peak = v
		}
		if dd := peak - v; dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}
