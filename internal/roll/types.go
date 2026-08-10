// Package roll builds continuous TX contract series from day-by-day contract
// snapshots (see internal/contract), using a pluggable Selector to decide
// which contract month is "front" each day, then back-adjusts prices so
// rolls don't appear as artificial gaps.
package roll

import (
	"time"

	"strategy/internal/contract"
)

// ContinuousBar is one day of the continuous series.
type ContinuousBar struct {
	Date          time.Time
	ContractMonth string // which raw contract this day's data came from
	RollDay       bool   // true if ContractMonth changed from the previous bar

	// Raw (unadjusted) prices from the selected front contract. Close is
	// SettlementPrice if present, else Last.
	Open, High, Low, Close *float64

	// Back-adjusted prices: the most recent contract segment is left equal
	// to the raw price; earlier segments are shifted so the series has no
	// artificial roll-day gaps. See backAdjust in build.go.
	AdjustedOpen, AdjustedHigh, AdjustedLow, AdjustedClose *float64

	Volume       int64
	OpenInterest *int64
}

// Selector chooses which contract month is "front" for a given day. Next is
// called once per day in ascending date order; implementations are stateful
// (they remember the previously chosen month).
type Selector interface {
	Next(day contract.DaySnapshot) (contractMonth string, err error)
}
