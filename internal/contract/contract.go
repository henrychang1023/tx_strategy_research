// Package contract groups parsed TX futures bars by trading day and attaches
// each contract month's resolved settlement date, giving roll logic a
// ready-to-walk, front-to-back ordered view of what was tradeable each day.
package contract

import (
	"sort"
	"time"

	"strategy/internal/expiry"
	"strategy/internal/parser"
)

// MonthSnapshot is one contract month's Regular-session bar on a given day.
type MonthSnapshot struct {
	ContractMonth  string
	SettlementDate time.Time
	Bar            parser.FutureBar
}

// DaySnapshot is every live TX contract month's Regular-session bar for one
// trading day, with Months sorted by SettlementDate ascending (Months[0] is
// the nearest-to-expiry contract, i.e. the naive front month).
type DaySnapshot struct {
	Date   time.Time
	Months []MonthSnapshot
}

// GroupByDay filters bars to Regular session and groups them by day, with
// each month's settlement date resolved by settlementDates. Days are
// returned sorted ascending; Months within a day are sorted by settlement
// date ascending.
func GroupByDay(bars []parser.FutureBar) ([]DaySnapshot, error) {
	regular := make([]parser.FutureBar, 0, len(bars))
	for _, b := range bars {
		if b.TradingSession == "Regular" {
			regular = append(regular, b)
		}
	}

	settleOf, err := settlementDates(regular)
	if err != nil {
		return nil, err
	}

	byDate := make(map[time.Time][]MonthSnapshot)
	for _, b := range regular {
		byDate[b.Date] = append(byDate[b.Date], MonthSnapshot{
			ContractMonth:  b.ContractMonth,
			SettlementDate: settleOf[b.ContractMonth],
			Bar:            b,
		})
	}

	days := make([]DaySnapshot, 0, len(byDate))
	for date, months := range byDate {
		sort.Slice(months, func(i, j int) bool {
			return months[i].SettlementDate.Before(months[j].SettlementDate)
		})
		days = append(days, DaySnapshot{Date: date, Months: months})
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Date.Before(days[j].Date) })
	return days, nil
}

// settlementDates resolves each contract month's true settlement date. TX
// settlement is nominally the third Wednesday of the delivery month
// (internal/expiry), but a multi-day holiday closure can push it later —
// e.g. Lunar New Year shifted 202301's settlement from the theoretical
// 2023-01-18 to the actual 2023-01-30, confirmed against the raw data. The
// data marks the real settlement day unambiguously: settlement_price is
// exactly 0 on that day only (see 台指期無縫轉倉研究計畫.md's known-caveats
// section), so that day is used as ground truth whenever a contract has
// reached it within the dataset. Contracts that have not yet settled (still
// trading at the end of the dataset) fall back to the theoretical date; this
// only affects ordering since such months never reach a roll cutoff.
func settlementDates(regular []parser.FutureBar) (map[string]time.Time, error) {
	zeroSettle := make(map[string]time.Time)
	months := make(map[string]bool)
	for _, b := range regular {
		months[b.ContractMonth] = true
		if b.SettlementPrice != nil && *b.SettlementPrice == 0 {
			zeroSettle[b.ContractMonth] = b.Date
		}
	}

	settle := make(map[string]time.Time, len(months))
	for cm := range months {
		if d, ok := zeroSettle[cm]; ok {
			settle[cm] = d
			continue
		}
		theoretical, err := expiry.SettlementDate(cm)
		if err != nil {
			return nil, err
		}
		settle[cm] = theoretical
	}
	return settle, nil
}
