// simulate2 answers 模擬策略v2.md: starting from a fixed NT$1,000,000
// capital base, run the fixed-roll-method endless-rolling strategy at four
// leverage levels (0.75x/1x/1.25x/1.5x — notional exposure at entry relative
// to capital, held fixed thereafter with no rebalancing, same simplification
// as every prior phase), plus a TAIEX Total Return Index control (buying
// NT$1,000,000 of the index outright), and reports month-by-month total
// gain and monthly return rate for all five.
//
// Leverage here means position size, not StartingCapital: internal/engine's
// Params.ContractMultiplier (added for this) scales both P&L and cost as if
// holding that many TX contracts, while StartingCapital stays NT$1,000,000
// throughout — unlike Phase 3/5, where "leverage" scenarios instead varied
// StartingCapital against a fixed 1-contract position.
package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"time"

	"strategy/internal/cleaner"
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

const (
	taiexFile       = "data/raw/twse/Taiwan Stock Exchange Capitalization Weighted Stock Index.csv"
	startingCapital = 1_000_000.0
	outCSV          = "output/backtest/simulate2_monthly.csv"
)

var leverages = []float64{0.75, 1.0, 1.25, 1.5}

// series is one scenario's cumulative NetPnL, keyed by trading date.
type series struct {
	name   string
	byDate map[string]float64
	dates  []time.Time
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
	cbars, err := roll.Build(days, roll.NewFixedSelector(days))
	if err != nil {
		log.Fatalf("build fixed continuous series: %v", err)
	}

	indexBars, err := parser.LoadTAIEXCSV(taiexFile)
	if err != nil {
		log.Fatalf("load %s: %v", taiexFile, err)
	}
	indexBars = cleaner.SortIndex(indexBars)

	window := trimToTAIEXWindow(cbars, indexBars[0].Date, indexBars[len(indexBars)-1].Date)
	entryPrice := *window[0].Close

	var all []series

	cost := engine.DefaultCostModel
	for _, lev := range leverages {
		contractMultiplier := lev * startingCapital / (entryPrice * engine.PointValue)
		initialMargin := contractMultiplier * engine.DefaultInitialMargin
		maintenanceMargin := initialMargin * engine.DefaultMaintenanceRatio

		result := engine.RunFundMode(window, engine.Params{
			Cost:               cost,
			StartingCapital:    startingCapital,
			InitialMargin:      initialMargin,
			MaintenanceMargin:  maintenanceMargin,
			ContractMultiplier: contractMultiplier,
		})

		s := series{name: fmt.Sprintf("%.2fx", lev), byDate: map[string]float64{}}
		for i, d := range result.Dates {
			key := d.Format("2006-01-02")
			s.byDate[key] = result.NetPnL[i]
			s.dates = append(s.dates, d)
		}
		fmt.Printf("%s: 期末總收益 NT$%.0f，追繳次數 %d，累積追繳 NT$%.0f\n",
			s.name, result.NetPnL[len(result.NetPnL)-1], len(result.MarginCalls), result.TotalInjectedCapital)
		all = append(all, s)
	}

	// TAIEX control: buy NT$1,000,000 of the Total Return Index outright on
	// the entry day, no leverage.
	indexByDate := make(map[string]parser.IndexBar, len(indexBars))
	for _, b := range indexBars {
		indexByDate[b.Date.Format("2006-01-02")] = b
	}
	entryTRI := indexByDate[window[0].Date.Format("2006-01-02")].TotalReturnIndex
	units := startingCapital / entryTRI
	taiex := series{name: "TAIEX", byDate: map[string]float64{}}
	for _, b := range window {
		key := b.Date.Format("2006-01-02")
		taiex.byDate[key] = units*indexByDate[key].TotalReturnIndex - startingCapital
		taiex.dates = append(taiex.dates, b.Date)
	}
	fmt.Printf("%s: 期末總收益 NT$%.0f\n\n", taiex.name, taiex.byDate[window[len(window)-1].Date.Format("2006-01-02")])

	all = append(all, taiex)

	months := monthEnds(window)
	writeCSV(months, all)
	printMarkdownTables(months, all)
}

func trimToTAIEXWindow(cbars []roll.ContinuousBar, start, end time.Time) []roll.ContinuousBar {
	lo := -1
	for i, b := range cbars {
		if lo == -1 && !b.Date.Before(start) {
			lo = i
		}
		if !b.Date.After(end) {
			continue
		}
		if lo != -1 {
			return cbars[lo:i]
		}
	}
	if lo == -1 {
		log.Fatal("no overlap between TX and TAIEX date ranges")
	}
	return cbars[lo:]
}

// monthEnds returns, for each calendar month present in window, the last
// trading date in that month.
func monthEnds(window []roll.ContinuousBar) []time.Time {
	lastOfMonth := make(map[string]time.Time)
	var order []string
	for _, b := range window {
		key := b.Date.Format("2006-01")
		if _, ok := lastOfMonth[key]; !ok {
			order = append(order, key)
		}
		lastOfMonth[key] = b.Date
	}
	sort.Strings(order)
	out := make([]time.Time, len(order))
	for i, k := range order {
		out[i] = lastOfMonth[k]
	}
	return out
}

func writeCSV(months []time.Time, all []series) {
	f, err := os.Create(outCSV)
	if err != nil {
		log.Fatalf("create %s: %v", outCSV, err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"month"}
	for _, s := range all {
		header = append(header, s.name+"_net_pnl", s.name+"_monthly_return_pct")
	}
	if err := w.Write(header); err != nil {
		log.Fatalf("write header: %v", err)
	}

	prevPnL := make([]float64, len(all))
	for _, m := range months {
		row := []string{m.Format("2006-01")}
		key := m.Format("2006-01-02")
		for i, s := range all {
			pnl := s.byDate[key]
			monthlyReturn := (pnl - prevPnL[i]) / startingCapital * 100
			row = append(row, strconv.FormatFloat(pnl, 'f', 0, 64), strconv.FormatFloat(monthlyReturn, 'f', 3, 64))
			prevPnL[i] = pnl
		}
		if err := w.Write(row); err != nil {
			log.Fatalf("write row: %v", err)
		}
	}
	fmt.Printf("wrote %s (%d months)\n\n", outCSV, len(months))
}

func printMarkdownTables(months []time.Time, all []series) {
	fmt.Println("### 逐月總收益 (NT$)")
	printHeader(all)
	prevPnL := make([]float64, len(all))
	for _, m := range months {
		key := m.Format("2006-01-02")
		row := m.Format("2006-01")
		for i, s := range all {
			pnl := s.byDate[key]
			row += fmt.Sprintf(" | %.0f", pnl)
			prevPnL[i] = pnl
		}
		fmt.Println(row)
	}

	fmt.Println("\n### 逐月報酬率 (%)")
	printHeader(all)
	prevPnL = make([]float64, len(all))
	for _, m := range months {
		key := m.Format("2006-01-02")
		row := m.Format("2006-01")
		for i, s := range all {
			pnl := s.byDate[key]
			monthlyReturn := (pnl - prevPnL[i]) / startingCapital * 100
			row += fmt.Sprintf(" | %.2f%%", monthlyReturn)
			prevPnL[i] = pnl
		}
		fmt.Println(row)
	}
}

func printHeader(all []series) {
	header := "月份"
	sep := "---"
	for _, s := range all {
		header += " | " + s.name
		sep += " | ---"
	}
	fmt.Println(header)
	fmt.Println(sep)
}
