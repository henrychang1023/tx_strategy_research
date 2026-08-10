// scenarios is the Phase 5 step: runs the full parameter grid (roll method x
// trailing-stop x cost assumption x, for fund mode, leverage) over the
// buy-and-hold backtest from Phase 3, writing two summary CSVs. See
// 台指期無縫轉倉研究計畫.md Phase 5 and
// C:\Users\User\.claude\plans\bright-watching-cerf.md for the design — in
// particular why a trailing stop with no re-entry needs no changes to
// internal/roll or internal/engine: it's just a prefix slice of the already
// back-adjusted continuous series.
package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"

	"strategy/internal/contract"
	"strategy/internal/engine"
	"strategy/internal/parser"
	"strategy/internal/roll"
	"strategy/internal/strategy"
)

var futureFiles = []string{
	"data/raw/taifex/2020_fut.csv",
	"data/raw/taifex/2021_fut.csv",
	"data/raw/taifex/2022_fut.csv",
	"data/raw/taifex/2023_fut.csv",
	"data/raw/taifex/2024_fut.csv",
	"data/raw/taifex/2025_fut.csv",
}

const outDir = "output/backtest"

var stopScenarios = []struct {
	name string
	pct  float64
	none bool
}{
	{name: "none", none: true},
	{name: "trail_5pct", pct: 0.05},
	{name: "trail_10pct", pct: 0.10},
	{name: "trail_15pct", pct: 0.15},
}

var costScenarios = []struct {
	name string
	cost engine.CostModel
}{
	{"low", engine.CostModel{CommissionPerContract: 30, TaxRate: 0.00002, SlippagePoints: 0.5}},
	{"medium", engine.DefaultCostModel},
	{"high", engine.CostModel{CommissionPerContract: 100, TaxRate: 0.00002, SlippagePoints: 3}},
}

var leverageScenarios = []struct {
	name       string
	marginMult float64
}{
	{"aggressive_1.2x_margin", 1.2},
	{"moderate_2x_margin", 2},
	{"conservative_4x_margin", 4},
}

func main() {
	futBars, err := parser.LoadFutureYears(futureFiles, "TX")
	if err != nil {
		log.Fatalf("load future data: %v", err)
	}
	days, err := contract.GroupByDay(futBars)
	if err != nil {
		log.Fatalf("group by day: %v", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", outDir, err)
	}

	rollMethods := []struct {
		name string
		sel  roll.Selector
	}{
		{"fixed", roll.NewFixedSelector(days)},
		{"volume", roll.NewVolumeSelector(days)},
		{"oi", roll.NewOISelector(days)},
	}

	initialMargin := engine.DefaultInitialMargin
	maintenanceMargin := initialMargin * engine.DefaultMaintenanceRatio

	pointsFile := outDir + "/phase5_points.csv"
	fundFile := outDir + "/phase5_fund.csv"

	pf, err := os.Create(pointsFile)
	if err != nil {
		log.Fatalf("create %s: %v", pointsFile, err)
	}
	defer pf.Close()
	pw := csv.NewWriter(pf)
	mustWrite(pw, []string{"roll_method", "stop_scenario", "cost_scenario",
		"held_days", "total_points", "sharpe", "max_dd_points", "total_cost_points"})

	ff, err := os.Create(fundFile)
	if err != nil {
		log.Fatalf("create %s: %v", fundFile, err)
	}
	defer ff.Close()
	fw := csv.NewWriter(ff)
	mustWrite(fw, []string{"roll_method", "stop_scenario", "cost_scenario", "leverage_scenario",
		"held_days", "return_pct", "sharpe", "max_dd_pct", "margin_calls", "injected_capital"})

	// highlights[stopScenario] = fixed-method point result at cost=medium,
	// printed at the end as the headline stop-loss comparison.
	highlights := make(map[string]engine.PointResult)

	for _, rm := range rollMethods {
		cbars, err := roll.Build(days, rm.sel)
		if err != nil {
			log.Fatalf("build %s: %v", rm.name, err)
		}

		for _, stop := range stopScenarios {
			exitIdx := len(cbars) - 1
			if !stop.none {
				exitIdx = strategy.TrailingStopExitIndex(cbars, stop.pct)
			}
			held := cbars[:exitIdx+1]

			for _, cs := range costScenarios {
				pointRes := engine.RunPointMode(held, cs.cost)
				mustWrite(pw, []string{
					rm.name, stop.name, cs.name,
					strconv.Itoa(len(held)),
					f2(pointRes.CumulativePoints[len(pointRes.CumulativePoints)-1]),
					f2(pointRes.Sharpe),
					f2(pointRes.MaxDrawdownPoints),
					f2(pointRes.TotalCostPoints),
				})
				if rm.name == "fixed" && cs.name == "medium" {
					highlights[stop.name] = pointRes
				}

				for _, lev := range leverageScenarios {
					params := engine.Params{
						Cost:              cs.cost,
						StartingCapital:   initialMargin * lev.marginMult,
						InitialMargin:     initialMargin,
						MaintenanceMargin: maintenanceMargin,
					}
					fundRes := engine.RunFundMode(held, params)
					mustWrite(fw, []string{
						rm.name, stop.name, cs.name, lev.name,
						strconv.Itoa(len(held)),
						f2(fundRes.ReturnPct[len(fundRes.ReturnPct)-1] * 100),
						f2(fundRes.Sharpe),
						f2(fundRes.MaxDrawdownPct * 100),
						strconv.Itoa(len(fundRes.MarginCalls)),
						f2(fundRes.TotalInjectedCapital),
					})
				}
			}
		}
	}

	pw.Flush()
	fw.Flush()
	if err := pw.Error(); err != nil {
		log.Fatalf("write %s: %v", pointsFile, err)
	}
	if err := fw.Error(); err != nil {
		log.Fatalf("write %s: %v", fundFile, err)
	}
	fmt.Printf("wrote %s (36 rows), %s (108 rows)\n\n", pointsFile, fundFile)

	fmt.Println("=== 停損情境比較（fixed 轉倉法、medium 成本）===")
	fmt.Printf("%-14s %10s %12s %8s %14s\n", "情境", "持有天數", "總報酬(點)", "Sharpe", "最大回撤(點)")
	for _, stop := range stopScenarios {
		r := highlights[stop.name]
		fmt.Printf("%-14s %10d %12.1f %8.2f %14.1f\n",
			stop.name, len(r.Dates), r.CumulativePoints[len(r.CumulativePoints)-1], r.Sharpe, r.MaxDrawdownPoints)
	}
}

func f2(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func mustWrite(w *csv.Writer, row []string) {
	if err := w.Write(row); err != nil {
		log.Fatalf("write csv row: %v", err)
	}
}
