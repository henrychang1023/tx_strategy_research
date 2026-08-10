package engine

import (
	"time"

	"strategy/internal/roll"
)

// PointResult is the pure-points (純點數報酬) accounting of a buy-and-hold
// backtest: comparable directly to TAIEX's own point return, ignoring
// margin/leverage entirely.
type PointResult struct {
	Dates             []time.Time
	DailyPoints       []float64 // day-over-day adjusted-close change, minus that day's trading costs
	CumulativePoints  []float64
	Sharpe            float64
	MaxDrawdownPoints float64
	TotalCostPoints   float64
}

// RunPointMode simulates holding 1 long contract from the first bar to the
// last, in index points, after subtracting entry/roll/exit trading costs.
func RunPointMode(bars []roll.ContinuousBar, cost CostModel) PointResult {
	n := len(bars)
	res := PointResult{
		Dates:            make([]time.Time, n),
		DailyPoints:      make([]float64, n),
		CumulativePoints: make([]float64, n),
	}

	var cumulative float64
	for i, b := range bars {
		res.Dates[i] = b.Date

		costPoints := float64(costSides(bars, i)) * cost.PerSidePoints(*b.Close)
		res.TotalCostPoints += costPoints

		var move float64
		if i > 0 {
			move = *bars[i].AdjustedClose - *bars[i-1].AdjustedClose
		}

		daily := move - costPoints
		cumulative += daily
		res.DailyPoints[i] = daily
		res.CumulativePoints[i] = cumulative
	}

	res.Sharpe = sharpe(res.DailyPoints)
	res.MaxDrawdownPoints = maxDrawdown(res.CumulativePoints)
	return res
}

// costSides returns how many trade sides (one buy or one sell) happen on
// bars[i]: 1 for the initial entry, 2 for a roll (close old + open new), 1
// for the final exit. These add up (e.g. a roll landing on the last day is
// 3 sides: close old, open new, then immediately exit).
func costSides(bars []roll.ContinuousBar, i int) int {
	n := len(bars)
	sides := 0
	if i == 0 {
		sides++
	}
	if bars[i].RollDay {
		sides += 2
	}
	if i == n-1 {
		sides++
	}
	return sides
}
