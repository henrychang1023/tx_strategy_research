package roll

import (
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
func i64(v int64) *int64   { return &v }

func snapshot(t *testing.T, date, contractMonth, settle string, close float64, volume int64) contract.MonthSnapshot {
	t.Helper()
	return contract.MonthSnapshot{
		ContractMonth:  contractMonth,
		SettlementDate: mustDate(t, settle),
		Bar: parser.FutureBar{
			Date:            mustDate(t, date),
			Contract:        "TX",
			ContractMonth:   contractMonth,
			TradingSession:  "Regular",
			Last:            f(close),
			SettlementPrice: f(close),
			Volume:          volume,
			OpenInterest:    i64(volume), // reuse volume as OI in these fixtures; only value matters, not realism
		},
	}
}

// TestBuildFixedRoll uses A ("202001", settles 2020-01-15) and B ("202002",
// far enough out that it never hits a cutoff in this window) across 3 days
// spanning A's roll-out. A's 2020-01-15 close (999) is intentionally
// implausible to prove the fixed selector/back-adjust never reads it: real
// TAIFEX data has an unreliable settlement_price on a contract's actual
// final settlement day, which is exactly why the roll happens one trading
// day earlier.
func TestBuildFixedRoll(t *testing.T) {
	days := []contract.DaySnapshot{
		{Date: mustDate(t, "2020-01-13"), Months: []contract.MonthSnapshot{
			snapshot(t, "2020-01-13", "202001", "2020-01-15", 100, 10),
		}},
		{Date: mustDate(t, "2020-01-14"), Months: []contract.MonthSnapshot{
			snapshot(t, "2020-01-14", "202001", "2020-01-15", 101, 10),
			snapshot(t, "2020-01-14", "202002", "2020-02-19", 150, 5),
		}},
		{Date: mustDate(t, "2020-01-15"), Months: []contract.MonthSnapshot{
			snapshot(t, "2020-01-15", "202001", "2020-01-15", 999, 1), // must never be read
			snapshot(t, "2020-01-15", "202002", "2020-02-19", 152, 5),
		}},
	}

	sel := NewFixedSelector(days)
	bars, err := Build(days, sel)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(bars) != 3 {
		t.Fatalf("got %d bars, want 3", len(bars))
	}

	wantCM := []string{"202001", "202002", "202002"}
	wantRoll := []bool{false, true, false}
	wantAdjustedClose := []float64{149, 150, 152} // see hand-derivation in plan notes
	for idx, b := range bars {
		if b.ContractMonth != wantCM[idx] {
			t.Errorf("bars[%d].ContractMonth = %s, want %s", idx, b.ContractMonth, wantCM[idx])
		}
		if b.RollDay != wantRoll[idx] {
			t.Errorf("bars[%d].RollDay = %v, want %v", idx, b.RollDay, wantRoll[idx])
		}
		if b.AdjustedClose == nil || *b.AdjustedClose != wantAdjustedClose[idx] {
			t.Errorf("bars[%d].AdjustedClose = %v, want %v", idx, b.AdjustedClose, wantAdjustedClose[idx])
		}
	}

	// Most recent day must equal raw (backward/Panama convention).
	last := bars[len(bars)-1]
	if *last.AdjustedClose != *last.Close {
		t.Errorf("last bar AdjustedClose = %v, want raw Close = %v", *last.AdjustedClose, *last.Close)
	}

	// No artificial jump across the roll: the adjusted move must equal the
	// old contract's own move over the same two days (101-100=1), not the
	// raw cross-contract gap (150-100=50).
	gotMove := *bars[1].AdjustedClose - *bars[0].AdjustedClose
	if gotMove != 1 {
		t.Errorf("adjusted move across roll = %v, want 1 (old contract's own day-over-day move)", gotMove)
	}
}

func TestVolumeSelectorSwitchesOnCrossover(t *testing.T) {
	// Settlement dates far outside this 4-day window so no forced cutoff
	// applies; the switch must be driven purely by volume.
	days := []contract.DaySnapshot{
		{Date: mustDate(t, "2020-01-13"), Months: []contract.MonthSnapshot{
			snapshot(t, "2020-01-13", "202006", "2020-06-17", 100, 100),
			snapshot(t, "2020-01-13", "202009", "2020-09-16", 200, 50),
		}},
		{Date: mustDate(t, "2020-01-14"), Months: []contract.MonthSnapshot{
			snapshot(t, "2020-01-14", "202006", "2020-06-17", 101, 100),
			snapshot(t, "2020-01-14", "202009", "2020-09-16", 201, 50),
		}},
		{Date: mustDate(t, "2020-01-15"), Months: []contract.MonthSnapshot{
			snapshot(t, "2020-01-15", "202006", "2020-06-17", 102, 50),
			snapshot(t, "2020-01-15", "202009", "2020-09-16", 202, 120),
		}},
		{Date: mustDate(t, "2020-01-16"), Months: []contract.MonthSnapshot{
			snapshot(t, "2020-01-16", "202006", "2020-06-17", 103, 50),
			snapshot(t, "2020-01-16", "202009", "2020-09-16", 203, 120),
		}},
	}

	sel := NewVolumeSelector(days)
	bars, err := Build(days, sel)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	wantCM := []string{"202006", "202006", "202009", "202009"}
	wantRoll := []bool{false, false, true, false}
	for idx, b := range bars {
		if b.ContractMonth != wantCM[idx] {
			t.Errorf("bars[%d].ContractMonth = %s, want %s", idx, b.ContractMonth, wantCM[idx])
		}
		if b.RollDay != wantRoll[idx] {
			t.Errorf("bars[%d].RollDay = %v, want %v", idx, b.RollDay, wantRoll[idx])
		}
	}
}

func TestVolumeSelectorForcedCutoffOverridesVolume(t *testing.T) {
	// A's volume dominates B's throughout, so a pure volume comparison would
	// never switch. The forced roll-cutoff (one trading day before A's
	// settlement) must still kick in.
	days := []contract.DaySnapshot{
		{Date: mustDate(t, "2020-01-13"), Months: []contract.MonthSnapshot{
			snapshot(t, "2020-01-13", "202001", "2020-01-15", 100, 9999),
		}},
		{Date: mustDate(t, "2020-01-14"), Months: []contract.MonthSnapshot{
			snapshot(t, "2020-01-14", "202001", "2020-01-15", 101, 9999),
			snapshot(t, "2020-01-14", "202002", "2020-02-19", 150, 1),
		}},
		{Date: mustDate(t, "2020-01-15"), Months: []contract.MonthSnapshot{
			snapshot(t, "2020-01-15", "202001", "2020-01-15", 999, 9999), // unreliable; must not affect selection
			snapshot(t, "2020-01-15", "202002", "2020-02-19", 152, 1),
		}},
	}

	sel := NewVolumeSelector(days)
	bars, err := Build(days, sel)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if bars[1].ContractMonth != "202002" {
		t.Errorf("bars[1].ContractMonth = %s, want 202002 (forced cutoff should override volume)", bars[1].ContractMonth)
	}
	if !bars[1].RollDay {
		t.Errorf("bars[1].RollDay = false, want true")
	}
}
