// continuous is the Phase 2 step: it builds TX continuous contract series
// under three roll methods (fixed day-before-expiry, Volume switch, Open
// Interest switch), back-adjusts each for roll-day gaps, and writes them to
// data/clean/continuous/. See 台指期無縫轉倉研究計畫.md Phase 2 and
// C:\Users\User\.claude\plans\bright-watching-cerf.md for the design.
package main

import (
	"fmt"
	"log"
	"os"

	"strategy/internal/contract"
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

const outDir = "data/clean/continuous"

func main() {
	bars, err := parser.LoadFutureYears(futureFiles, "TX")
	if err != nil {
		log.Fatalf("load future data: %v", err)
	}

	days, err := contract.GroupByDay(bars)
	if err != nil {
		log.Fatalf("group by day: %v", err)
	}
	fmt.Printf("trading days: %d (%s -> %s)\n\n",
		len(days), days[0].Date.Format("2006-01-02"), days[len(days)-1].Date.Format("2006-01-02"))

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", outDir, err)
	}

	methods := []struct {
		name string
		sel  roll.Selector
		file string
	}{
		{"fixed", roll.NewFixedSelector(days), outDir + "/fixed.csv"},
		{"volume", roll.NewVolumeSelector(days), outDir + "/volume.csv"},
		{"oi", roll.NewOISelector(days), outDir + "/oi.csv"},
	}

	for _, m := range methods {
		bars, err := roll.Build(days, m.sel)
		if err != nil {
			log.Fatalf("build %s: %v", m.name, err)
		}
		if err := roll.WriteCSV(m.file, bars); err != nil {
			log.Fatalf("write %s: %v", m.file, err)
		}
		printSummary(m.name, m.file, bars)
	}
}

func printSummary(name, file string, bars []roll.ContinuousBar) {
	fmt.Printf("=== %s (%s) ===\n", name, file)
	fmt.Printf("rows: %d, range: %s -> %s\n",
		len(bars), bars[0].Date.Format("2006-01-02"), bars[len(bars)-1].Date.Format("2006-01-02"))

	rolls := 0
	for i, b := range bars {
		if !b.RollDay {
			continue
		}
		rolls++
		prev := bars[i-1]
		fmt.Printf("  roll %s: %s -> %s (raw close %.2f -> %.2f, adjusted %.2f -> %.2f)\n",
			b.Date.Format("2006-01-02"), prev.ContractMonth, b.ContractMonth,
			deref(prev.Close), deref(b.Close), deref(prev.AdjustedClose), deref(b.AdjustedClose))
	}
	fmt.Printf("total rolls: %d\n\n", rolls)
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
