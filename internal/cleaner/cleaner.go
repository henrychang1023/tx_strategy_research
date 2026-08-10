// Package cleaner sorts parsed data into a canonical order and writes it out
// as clean, plain-format CSVs (unified date format, sorted, no raw-file
// quirks). It never modifies data/raw; it only reads parser output and
// writes to data/clean.
package cleaner

import (
	"sort"

	"strategy/internal/parser"
)

// SortFuture returns bars sorted by date, then contract month, then session
// (Regular before After-Hours), ascending.
func SortFuture(bars []parser.FutureBar) []parser.FutureBar {
	out := make([]parser.FutureBar, len(bars))
	copy(out, bars)
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.Date.Equal(b.Date) {
			return a.Date.Before(b.Date)
		}
		if a.ContractMonth != b.ContractMonth {
			return a.ContractMonth < b.ContractMonth
		}
		return sessionRank(a.TradingSession) < sessionRank(b.TradingSession)
	})
	return out
}

func sessionRank(session string) int {
	if session == "Regular" {
		return 0
	}
	return 1 // After-Hours, or anything unexpected, sorts after Regular
}

// SortIndex returns bars sorted by date ascending.
func SortIndex(bars []parser.IndexBar) []parser.IndexBar {
	out := make([]parser.IndexBar, len(bars))
	copy(out, bars)
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}
