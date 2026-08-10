// backtest is the Phase 3 step: a buy-and-hold-1-contract backtest over each
// Phase 2 continuous contract (fixed/volume/oi roll methods), in both the
// pure-points and simulated-margin-fund accounting modes. See
// 台指期無縫轉倉研究計畫.md Phase 3 and
// C:\Users\User\.claude\plans\bright-watching-cerf.md for the design and the
// assumptions behind the default cost/margin parameters.
package main

import (
	"fmt"
	"log"
	"os"

	"strategy/internal/contract"
	"strategy/internal/engine"
	"strategy/internal/parser"
	"strategy/internal/roll"
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

// leverageScenarios express StartingCapital as a multiple of InitialMargin:
// illustrative examples of how leverage shapes risk/return, not a
// recommendation.
var leverageScenarios = []struct {
	name       string
	marginMult float64
}{
	{"aggressive_1.2x_margin", 1.2},
	{"moderate_2x_margin", 2},
	{"conservative_4x_margin", 4},
}

func main() {
	bars, err := parser.LoadFutureYears(futureFiles, "TX")
	if err != nil {
		log.Fatalf("load future data: %v", err)
	}
	days, err := contract.GroupByDay(bars)
	if err != nil {
		log.Fatalf("group by day: %v", err)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", outDir, err)
	}

	cost := engine.DefaultCostModel
	initialMargin := engine.DefaultInitialMargin
	maintenanceMargin := initialMargin * engine.DefaultMaintenanceRatio

	methods := []struct {
		name string
		sel  roll.Selector
	}{
		{"fixed", roll.NewFixedSelector(days)},
		{"volume", roll.NewVolumeSelector(days)},
		{"oi", roll.NewOISelector(days)},
	}

	for _, m := range methods {
		cbars, err := roll.Build(days, m.sel)
		if err != nil {
			log.Fatalf("build %s: %v", m.name, err)
		}

		pointRes := engine.RunPointMode(cbars, cost)
		pointFile := fmt.Sprintf("%s/%s_points.csv", outDir, m.name)
		if err := engine.WritePointCSV(pointFile, pointRes); err != nil {
			log.Fatalf("write %s: %v", pointFile, err)
		}

		var fundScenarios []engine.NamedFundResult
		for _, sc := range leverageScenarios {
			params := engine.Params{
				Cost:              cost,
				StartingCapital:   initialMargin * sc.marginMult,
				InitialMargin:     initialMargin,
				MaintenanceMargin: maintenanceMargin,
			}
			fundScenarios = append(fundScenarios, engine.NamedFundResult{
				Scenario: sc.name,
				Result:   engine.RunFundMode(cbars, params),
			})
		}
		fundFile := fmt.Sprintf("%s/%s_fund.csv", outDir, m.name)
		if err := engine.WriteFundCSV(fundFile, fundScenarios); err != nil {
			log.Fatalf("write %s: %v", fundFile, err)
		}

		printSummary(m.name, pointRes, fundScenarios)
	}
}

func printSummary(name string, p engine.PointResult, fund []engine.NamedFundResult) {
	fmt.Printf("=== %s ===\n", name)
	fmt.Printf("points   : total=%.1f  sharpe=%.2f  maxDD=%.1f  totalCost=%.1f\n",
		p.CumulativePoints[len(p.CumulativePoints)-1], p.Sharpe, p.MaxDrawdownPoints, p.TotalCostPoints)
	for _, s := range fund {
		r := s.Result
		fmt.Printf("fund %-24s: return=%.1f%%  sharpe=%.2f  maxDD=%.1f%%  marginCalls=%d  injected=%.0f\n",
			s.Scenario, r.ReturnPct[len(r.ReturnPct)-1]*100, r.Sharpe, r.MaxDrawdownPct*100,
			len(r.MarginCalls), r.TotalInjectedCapital)
	}
	fmt.Println()
}
