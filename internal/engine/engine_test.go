package engine

import (
	"testing"
	"time"

	"strategy/internal/roll"
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

func TestCostModelConversions(t *testing.T) {
	cost := CostModel{CommissionPerContract: 50, TaxRate: 0.00002, SlippagePoints: 1}
	price := 20000.0

	wantPoints := 1 + 50.0/PointValue + price*0.00002 // 1 + 0.25 + 0.4 = 1.65
	if got := cost.PerSidePoints(price); got != wantPoints {
		t.Errorf("PerSidePoints(%v) = %v, want %v", price, got, wantPoints)
	}
	wantNT := wantPoints * PointValue // 330
	if got := cost.PerSideNT(price); got != wantNT {
		t.Errorf("PerSideNT(%v) = %v, want %v", price, got, wantNT)
	}
}

// bar builds a minimal ContinuousBar for engine tests: Close is the raw
// price used for cost calculation, AdjustedClose drives the point series.
func bar(t *testing.T, date string, close, adjustedClose float64, rollDay bool) roll.ContinuousBar {
	t.Helper()
	return roll.ContinuousBar{
		Date:          mustDate(t, date),
		Close:         f(close),
		AdjustedClose: f(adjustedClose),
		RollDay:       rollDay,
	}
}

// TestRunPointMode hand-verifies a 4-day series with one roll (day 2) using
// a cost model of exactly 1 point per side (no commission/tax) so the
// entry/roll/exit cost deductions are easy to check by hand:
//
//	day0 (entry, 1 side):  move=0,             cost=1 -> daily=-1,  cum=-1
//	day1 (plain):          move=100,           cost=0 -> daily=100, cum=99
//	day2 (roll, 2 sides):  move=150,           cost=2 -> daily=148, cum=247
//	day3 (exit, 1 side):   move=-50,           cost=1 -> daily=-51, cum=196
func TestRunPointMode(t *testing.T) {
	bars := []roll.ContinuousBar{
		bar(t, "2020-01-01", 10000, 9000, false),
		bar(t, "2020-01-02", 10100, 9100, false),
		bar(t, "2020-01-03", 10300, 9250, true),
		bar(t, "2020-01-06", 10250, 9200, false),
	}
	cost := CostModel{SlippagePoints: 1} // commission/tax = 0, isolates side-count math

	res := RunPointMode(bars, cost)

	wantDaily := []float64{-1, 100, 148, -51}
	wantCumulative := []float64{-1, 99, 247, 196}
	for i := range bars {
		if res.DailyPoints[i] != wantDaily[i] {
			t.Errorf("DailyPoints[%d] = %v, want %v", i, res.DailyPoints[i], wantDaily[i])
		}
		if res.CumulativePoints[i] != wantCumulative[i] {
			t.Errorf("CumulativePoints[%d] = %v, want %v", i, res.CumulativePoints[i], wantCumulative[i])
		}
	}
	if res.TotalCostPoints != 4 {
		t.Errorf("TotalCostPoints = %v, want 4 (1 entry + 2 roll + 1 exit sides)", res.TotalCostPoints)
	}
	if res.MaxDrawdownPoints != 51 {
		t.Errorf("MaxDrawdownPoints = %v, want 51 (peak 247 -> trough 196)", res.MaxDrawdownPoints)
	}
}

// TestRunFundModeMarginCall forces a large adverse move that blows through
// maintenance margin, verifying: the top-up restores Equity to exactly
// InitialMargin, NetPnL/ReturnPct reflect the full raw loss regardless of
// the top-up, and the event is recorded with the correct pre-top-up equity.
func TestRunFundModeMarginCall(t *testing.T) {
	bars := []roll.ContinuousBar{
		bar(t, "2020-01-01", 20000, 20000, false),
		bar(t, "2020-01-02", 17000, 17000, false), // -3000 points = -NT$600,000 on 1 contract
	}
	params := Params{
		Cost:              CostModel{}, // zero cost isolates the margin-call math
		StartingCapital:   400000,
		InitialMargin:     184000,
		MaintenanceMargin: 138000,
	}

	res := RunFundMode(bars, params)

	if len(res.MarginCalls) != 1 {
		t.Fatalf("got %d margin calls, want 1", len(res.MarginCalls))
	}
	call := res.MarginCalls[0]
	if call.EquityBefore != -200000 {
		t.Errorf("MarginCalls[0].EquityBefore = %v, want -200000 (400000 - 600000)", call.EquityBefore)
	}
	if call.TopUp != 384000 {
		t.Errorf("MarginCalls[0].TopUp = %v, want 384000 (184000 - (-200000))", call.TopUp)
	}
	if res.Equity[1] != params.InitialMargin {
		t.Errorf("Equity[1] = %v, want %v (topped back up to InitialMargin)", res.Equity[1], params.InitialMargin)
	}
	if res.NetPnL[1] != -600000 {
		t.Errorf("NetPnL[1] = %v, want -600000 (unaffected by the top-up)", res.NetPnL[1])
	}
	if res.ReturnPct[1] != -1.5 {
		t.Errorf("ReturnPct[1] = %v, want -1.5 (-600000/400000)", res.ReturnPct[1])
	}
	if res.TotalInjectedCapital != 384000 {
		t.Errorf("TotalInjectedCapital = %v, want 384000", res.TotalInjectedCapital)
	}
}
