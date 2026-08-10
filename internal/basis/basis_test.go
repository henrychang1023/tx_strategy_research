package basis

import (
	"math"
	"testing"
	"time"

	"strategy/internal/contract"
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

func f(v float64) *float64 { return &v }

func daySnapshot(t *testing.T, date string, months ...contract.MonthSnapshot) contract.DaySnapshot {
	t.Helper()
	return contract.DaySnapshot{Date: mustDate(t, date), Months: months}
}

func monthSnapshot(t *testing.T, date, contractMonth, settle string, close float64) contract.MonthSnapshot {
	t.Helper()
	return contract.MonthSnapshot{
		ContractMonth:  contractMonth,
		SettlementDate: mustDate(t, settle),
		Bar: parser.FutureBar{
			Date:            mustDate(t, date),
			Contract:        "TX",
			ContractMonth:   contractMonth,
			TradingSession:  "Regular",
			SettlementPrice: f(close),
		},
	}
}

func indexBar(t *testing.T, date string, priceIndex float64) parser.IndexBar {
	t.Helper()
	return parser.IndexBar{Date: mustDate(t, date), PriceIndex: priceIndex}
}

func TestComputeJoinsAndUsesNearestMonth(t *testing.T) {
	days := []contract.DaySnapshot{
		daySnapshot(t, "2021-01-04",
			monthSnapshot(t, "2021-01-04", "202101", "2021-01-20", 14750), // front (nearest settlement)
			monthSnapshot(t, "2021-01-04", "202102", "2021-02-17", 14800), // back month, must be ignored
		),
	}
	indexBars := []parser.IndexBar{indexBar(t, "2021-01-04", 14902.03)}

	bars, err := Compute(days, indexBars)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(bars) != 1 {
		t.Fatalf("got %d bars, want 1", len(bars))
	}
	b := bars[0]
	if b.ContractMonth != "202101" {
		t.Errorf("ContractMonth = %s, want 202101 (nearest settlement, not 202102)", b.ContractMonth)
	}
	if b.Future != 14750 {
		t.Errorf("Future = %v, want 14750", b.Future)
	}
	if b.Index != 14902.03 {
		t.Errorf("Index = %v, want 14902.03", b.Index)
	}
	wantBasis := -152.03
	if math.Abs(b.Basis-wantBasis) > 1e-9 {
		t.Errorf("Basis = %v, want %v", b.Basis, wantBasis)
	}
	if b.DaysToExpiry != 16 { // 2021-01-04 -> 2021-01-20
		t.Errorf("DaysToExpiry = %d, want 16", b.DaysToExpiry)
	}
}

func TestComputeSkipsDaysWithoutIndexMatch(t *testing.T) {
	days := []contract.DaySnapshot{
		daySnapshot(t, "2020-01-02", monthSnapshot(t, "2020-01-02", "202001", "2020-01-15", 12101)),
		daySnapshot(t, "2021-01-04", monthSnapshot(t, "2021-01-04", "202101", "2021-01-20", 14750)),
	}
	// TAIEX data starts 2021-01-04; 2020-01-02 has no match and must be skipped, not errored.
	indexBars := []parser.IndexBar{indexBar(t, "2021-01-04", 14902.03)}

	bars, err := Compute(days, indexBars)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(bars) != 1 {
		t.Fatalf("got %d bars, want 1 (2020-01-02 should be skipped)", len(bars))
	}
	if !bars[0].Date.Equal(mustDate(t, "2021-01-04")) {
		t.Errorf("bars[0].Date = %s, want 2021-01-04", bars[0].Date)
	}
}
