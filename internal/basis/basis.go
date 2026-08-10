// Package basis computes the daily gap between TX front-month futures and
// the TAIEX index (台指期無縫轉倉研究計畫.md Phase 4). Basis is a property of
// a real, tradable contract, so it always uses the actual front-month raw
// price — never the Phase 2 back-adjusted continuous series, which is a
// synthetic construct for backtesting, not a real quote.
package basis

import (
	"fmt"
	"time"

	"strategy/internal/contract"
	"strategy/internal/parser"
)

// Bar is one day's basis observation.
type Bar struct {
	Date          time.Time
	ContractMonth string  // front-month contract used for Future
	Future        float64 // front-month raw close (settlement_price, else last)
	Index         float64 // TAIEX Price Index
	Basis         float64 // Future - Index
	DaysToExpiry  int     // calendar days from Date to the front month's settlement date
}

// Compute joins each TX trading day's front-month contract (days[i].Months[0]
// — GroupByDay already sorts by settlement date ascending) against that
// day's TAIEX index level. Days without a matching index observation are
// skipped (TAIEX data starts 2021-01-04, TX data starts 2020-01-02 — see
// Phase 1 notes), not treated as an error.
func Compute(days []contract.DaySnapshot, indexBars []parser.IndexBar) ([]Bar, error) {
	indexByDate := make(map[time.Time]parser.IndexBar, len(indexBars))
	for _, b := range indexBars {
		indexByDate[b.Date] = b
	}

	bars := make([]Bar, 0, len(days))
	for _, day := range days {
		idx, ok := indexByDate[day.Date]
		if !ok {
			continue
		}
		if len(day.Months) == 0 {
			return nil, fmt.Errorf("%s: no contract months available", day.Date.Format("2006-01-02"))
		}
		front := day.Months[0]

		future, err := front.Bar.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", day.Date.Format("2006-01-02"), err)
		}

		bars = append(bars, Bar{
			Date:          day.Date,
			ContractMonth: front.ContractMonth,
			Future:        future,
			Index:         idx.PriceIndex,
			Basis:         future - idx.PriceIndex,
			DaysToExpiry:  int(front.SettlementDate.Sub(day.Date).Hours() / 24),
		})
	}
	return bars, nil
}
