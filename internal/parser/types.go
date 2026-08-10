// Package parser converts raw CSV data (TAIFEX futures, TWSE index) into
// typed Go structs. It only parses and filters to the relevant rows; it does
// not clean, roll, or otherwise reinterpret values (e.g. the known
// settlement_price == 0 on expiry days is left as-is here).
package parser

import (
	"fmt"
	"time"
)

// FutureBar is one row of TAIFEX daily futures data for a single contract
// month and trading session (Regular or After-Hours).
type FutureBar struct {
	Date          time.Time
	Contract      string // e.g. "TX"
	ContractMonth string // e.g. "202001" (YYYYMM)

	// Open, High, Low, Last, Change, ChangePercent, HistoricalHigh,
	// HistoricalLow are nil when the source field is "-", which happens on
	// zero-Volume rows (no trades that session).
	Open          *float64
	High          *float64
	Low           *float64
	Last          *float64
	Change        *float64
	ChangePercent *float64 // e.g. 0.90 for "0.90%"

	Volume int64

	// SettlementPrice, OpenInterest, BestBid, BestAsk are nil when the
	// source field is "-" (always the case for After-Hours rows).
	SettlementPrice *float64
	OpenInterest    *int64
	BestBid         *float64
	BestAsk         *float64

	HistoricalHigh *float64
	HistoricalLow  *float64

	TradingSession string // "Regular" or "After-Hours"
}

// Close returns the bar's canonical daily price: SettlementPrice if present
// and non-zero, else Last. SettlementPrice is literally 0 on a contract's
// actual final settlement day (see 台指期無縫轉倉研究計畫.md's known-caveats
// section) — that's a placeholder, not a real price, so it's treated the
// same as missing and Last (which does hold a real trade price that day) is
// used instead. Returns an error if neither is available, which happens on
// a zero-Volume day (see the Open/High/.../HistoricalLow doc comment above).
func (b FutureBar) Close() (float64, error) {
	if b.SettlementPrice != nil && *b.SettlementPrice != 0 {
		return *b.SettlementPrice, nil
	}
	if b.Last != nil {
		return *b.Last, nil
	}
	return 0, fmt.Errorf("contract %s has neither settlement_price nor last on %s",
		b.ContractMonth, b.Date.Format("2006-01-02"))
}

// IndexBar is one row of TWSE Capitalization Weighted Stock Index daily data.
type IndexBar struct {
	Date             time.Time
	PriceIndex       float64
	TotalReturnIndex float64
	Change           float64
	ChangePercent    float64
}
