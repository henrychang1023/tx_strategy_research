// Package expiry computes TAIEX futures (TX) final settlement days.
// The rule is fixed by TAIFEX: the third Wednesday of the contract's
// delivery month is the last trading day / settlement day. This is computed
// directly rather than read from data/raw/expiry/expiry.csv — that file is
// kept only as a historical cross-validation snapshot (see
// 台指期無縫轉倉研究計畫.md), not as a runtime data source.
package expiry

import (
	"fmt"
	"strconv"
	"time"
)

// ThirdWednesday returns the third Wednesday of the given year and month.
func ThirdWednesday(year int, month time.Month) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	offset := (int(time.Wednesday) - int(first.Weekday()) + 7) % 7
	firstWednesday := first.AddDate(0, 0, offset)
	return firstWednesday.AddDate(0, 0, 14)
}

// SettlementDate returns the final settlement day for a contract month
// string in "YYYYMM" form (as found in parser.FutureBar.ContractMonth), e.g.
// "202001" -> 2020-01-15.
func SettlementDate(contractMonth string) (time.Time, error) {
	if len(contractMonth) != 6 {
		return time.Time{}, fmt.Errorf("invalid contract month %q: want YYYYMM", contractMonth)
	}
	year, err := strconv.Atoi(contractMonth[:4])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid contract month %q: %w", contractMonth, err)
	}
	month, err := strconv.Atoi(contractMonth[4:6])
	if err != nil || month < 1 || month > 12 {
		return time.Time{}, fmt.Errorf("invalid contract month %q: bad month", contractMonth)
	}
	return ThirdWednesday(year, time.Month(month)), nil
}
