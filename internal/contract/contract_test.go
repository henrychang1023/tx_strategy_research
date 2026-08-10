package contract

import (
	"testing"
	"time"

	"strategy/internal/parser"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("bad test date %q: %v", s, err)
	}
	return d
}

func bar(t *testing.T, date, contractMonth, session string) parser.FutureBar {
	t.Helper()
	return parser.FutureBar{
		Date:           mustDate(t, date),
		Contract:       "TX",
		ContractMonth:  contractMonth,
		TradingSession: session,
	}
}

func f(v float64) *float64 { return &v }

func TestGroupByDay(t *testing.T) {
	bars := []parser.FutureBar{
		bar(t, "2020-01-02", "202001", "Regular"),
		bar(t, "2020-01-02", "202001", "After-Hours"),
		bar(t, "2020-01-02", "202002", "Regular"),
		bar(t, "2020-01-03", "202002", "Regular"),
		bar(t, "2020-01-03", "202001", "Regular"),
	}

	days, err := GroupByDay(bars)
	if err != nil {
		t.Fatalf("GroupByDay: %v", err)
	}
	if len(days) != 2 {
		t.Fatalf("got %d days, want 2", len(days))
	}

	// day order ascending
	if !days[0].Date.Equal(mustDate(t, "2020-01-02")) {
		t.Errorf("days[0].Date = %s, want 2020-01-02", days[0].Date)
	}
	if !days[1].Date.Equal(mustDate(t, "2020-01-03")) {
		t.Errorf("days[1].Date = %s, want 2020-01-03", days[1].Date)
	}

	// After-Hours row filtered out -> only 202001, 202002 Regular remain on day 0
	if len(days[0].Months) != 2 {
		t.Fatalf("days[0].Months has %d entries, want 2 (After-Hours should be filtered)", len(days[0].Months))
	}

	// Months sorted by settlement date ascending: 202001 (2020-01-15) before 202002 (2020-02-19)
	if days[0].Months[0].ContractMonth != "202001" {
		t.Errorf("days[0].Months[0].ContractMonth = %s, want 202001 (nearer settlement)", days[0].Months[0].ContractMonth)
	}
	if days[0].Months[1].ContractMonth != "202002" {
		t.Errorf("days[0].Months[1].ContractMonth = %s, want 202002", days[0].Months[1].ContractMonth)
	}

	want0115 := mustDate(t, "2020-01-15")
	if !days[0].Months[0].SettlementDate.Equal(want0115) {
		t.Errorf("202001 SettlementDate = %s, want %s", days[0].Months[0].SettlementDate, want0115)
	}
	want0219 := mustDate(t, "2020-02-19")
	if !days[0].Months[1].SettlementDate.Equal(want0219) {
		t.Errorf("202002 SettlementDate = %s, want %s", days[0].Months[1].SettlementDate, want0219)
	}

	// day 1 has months in reverse input order but should still sort ascending by settlement date
	if days[1].Months[0].ContractMonth != "202001" || days[1].Months[1].ContractMonth != "202002" {
		t.Errorf("days[1].Months order = [%s, %s], want [202001, 202002]",
			days[1].Months[0].ContractMonth, days[1].Months[1].ContractMonth)
	}
}

// TestGroupByDayHolidayShiftedSettlement reproduces 202301: its theoretical
// third-Wednesday settlement is 2023-01-18, but the market was closed for
// Lunar New Year and the real, data-confirmed settlement (marked by
// settlement_price == 0) landed on 2023-01-30. SettlementDate must resolve
// to the actual day, not the theoretical one.
func TestGroupByDayHolidayShiftedSettlement(t *testing.T) {
	normal := bar(t, "2023-01-17", "202301", "Regular")
	normal.SettlementPrice = f(14925)

	settlementDay := bar(t, "2023-01-30", "202301", "Regular")
	settlementDay.SettlementPrice = f(0)

	days, err := GroupByDay([]parser.FutureBar{normal, settlementDay})
	if err != nil {
		t.Fatalf("GroupByDay: %v", err)
	}

	want := mustDate(t, "2023-01-30")
	for _, d := range days {
		if got := d.Months[0].SettlementDate; !got.Equal(want) {
			t.Errorf("%s: SettlementDate = %s, want %s (actual, not theoretical 2023-01-18)",
				d.Date.Format("2006-01-02"), got.Format("2006-01-02"), want.Format("2006-01-02"))
		}
	}
}
