// clean is the Phase 1 data-cleaning step: it parses the raw TAIFEX futures
// and TWSE index CSVs under data/raw, sorts them into a canonical order, and
// writes unified-format CSVs under data/clean. Raw data under data/raw is
// never modified. See 台指期無縫轉倉研究計畫.md for the source data caveats
// this parser/cleaner pair accounts for.
package main

import (
	"fmt"
	"log"

	"strategy/internal/cleaner"
	"strategy/internal/parser"
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
	taiexFile      = "data/raw/twse/Taiwan Stock Exchange Capitalization Weighted Stock Index.csv"
	futureCleanOut = "data/clean/taifex_tx.csv"
	indexCleanOut  = "data/clean/twse_index.csv"
)

func main() {
	bars, err := parser.LoadFutureYears(futureFiles, "TX")
	if err != nil {
		log.Fatalf("load future data: %v", err)
	}
	bars = cleaner.SortFuture(bars)

	regular, afterHours := 0, 0
	for _, b := range bars {
		switch b.TradingSession {
		case "Regular":
			regular++
		case "After-Hours":
			afterHours++
		}
	}

	fmt.Println()
	fmt.Printf("TX total rows:   %d\n", len(bars))
	fmt.Printf("  Regular:       %d\n", regular)
	fmt.Printf("  After-Hours:   %d\n", afterHours)
	fmt.Printf("  date range:    %s -> %s\n",
		bars[0].Date.Format("2006-01-02"), bars[len(bars)-1].Date.Format("2006-01-02"))

	fmt.Println("\nfirst 3 rows:")
	for _, b := range bars[:3] {
		printBar(b)
	}
	fmt.Println("last 3 rows:")
	for _, b := range bars[len(bars)-3:] {
		printBar(b)
	}

	if err := cleaner.WriteFutureCSV(futureCleanOut, bars); err != nil {
		log.Fatalf("write %s: %v", futureCleanOut, err)
	}
	fmt.Printf("\nwrote %s\n", futureCleanOut)

	idx, err := parser.LoadTAIEXCSV(taiexFile)
	if err != nil {
		log.Fatalf("load %s: %v", taiexFile, err)
	}
	idx = cleaner.SortIndex(idx)

	fmt.Println()
	fmt.Printf("TAIEX rows:      %d\n", len(idx))
	fmt.Printf("  date range:    %s -> %s\n",
		idx[0].Date.Format("2006-01-02"), idx[len(idx)-1].Date.Format("2006-01-02"))
	fmt.Println("first row:", idx[0])
	fmt.Println("last row: ", idx[len(idx)-1])

	if err := cleaner.WriteIndexCSV(indexCleanOut, idx); err != nil {
		log.Fatalf("write %s: %v", indexCleanOut, err)
	}
	fmt.Printf("wrote %s\n", indexCleanOut)
}

func printBar(b parser.FutureBar) {
	fmt.Printf("  %s %-6s %-9s session=%-11s last=%-9s settlement=%-8s oi=%s\n",
		b.Date.Format("2006-01-02"), b.Contract, b.ContractMonth, b.TradingSession,
		fmtFloatPtr(b.Last), fmtFloatPtr(b.SettlementPrice), fmtIntPtr(b.OpenInterest))
}

func fmtFloatPtr(p *float64) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprintf("%.2f", *p)
}

func fmtIntPtr(p *int64) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", *p)
}
