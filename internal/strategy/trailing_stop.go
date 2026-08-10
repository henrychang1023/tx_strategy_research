// Package strategy turns a continuous contract series (internal/roll) into a
// held sub-period per some entry/exit rule, so internal/engine's existing
// point/fund accounting can run on that sub-period unchanged. See Phase 5
// notes in C:\Users\User\.claude\plans\bright-watching-cerf.md for why no
// re-entry logic is needed: a buy-and-hold entry with an exit rule and no
// re-entry always holds a single contiguous prefix of the full series.
package strategy

import "strategy/internal/roll"

// TrailingStopExitIndex returns the index of the last bar to hold under a
// buy-and-hold-with-trailing-stop rule: the day AdjustedClose first closes
// below (1-trailPct) times the running peak AdjustedClose seen so far, or
// len(bars)-1 if the stop never triggers (equivalent to Phase 3's plain
// buy-and-hold). AdjustedClose is used deliberately, not the raw Close,
// so a roll-day price gap between contracts never looks like a drawdown.
//
// Callers pass bars[:TrailingStopExitIndex(bars, trailPct)+1] to
// engine.RunPointMode/RunFundMode.
func TrailingStopExitIndex(bars []roll.ContinuousBar, trailPct float64) int {
	if len(bars) == 0 {
		return -1
	}
	peak := *bars[0].AdjustedClose
	for i, b := range bars {
		if *b.AdjustedClose > peak {
			peak = *b.AdjustedClose
		}
		if *b.AdjustedClose < peak*(1-trailPct) {
			return i
		}
	}
	return len(bars) - 1
}
