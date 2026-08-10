// basis is the Phase 4 step: joins TX front-month futures against the TAIEX
// index to compute daily Basis/DaysToExpiry, then reports convergence,
// contango/backwardation, and rolling-cost statistics. See
// 台指期無縫轉倉研究計畫.md Phase 4.
package main

import (
	"fmt"
	"log"
	"sort"

	"strategy/internal/basis"
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

const (
	taiexFile = "data/raw/twse/Taiwan Stock Exchange Capitalization Weighted Stock Index.csv"
	outFile   = "data/clean/basis.csv"
)

func main() {
	futBars, err := parser.LoadFutureYears(futureFiles, "TX")
	if err != nil {
		log.Fatalf("load future data: %v", err)
	}
	days, err := contract.GroupByDay(futBars)
	if err != nil {
		log.Fatalf("group by day: %v", err)
	}

	indexBars, err := parser.LoadTAIEXCSV(taiexFile)
	if err != nil {
		log.Fatalf("load %s: %v", taiexFile, err)
	}

	bars, err := basis.Compute(days, indexBars)
	if err != nil {
		log.Fatalf("compute basis: %v", err)
	}
	if err := basis.WriteCSV(outFile, bars); err != nil {
		log.Fatalf("write %s: %v", outFile, err)
	}
	fmt.Printf("wrote %s (%d rows, %s -> %s)\n\n",
		outFile, len(bars), bars[0].Date.Format("2006-01-02"), bars[len(bars)-1].Date.Format("2006-01-02"))

	printConvergence(bars)
	printContango(bars)
	printRollingCost(days, indexBars)
}

// printConvergence buckets by DaysToExpiry and shows mean |Basis| shrinking
// toward 0 as a contract approaches settlement.
func printConvergence(bars []basis.Bar) {
	type bucket struct {
		label  string
		lo, hi int // inclusive
		sum    float64
		absSum float64
		count  int
	}
	buckets := []*bucket{
		{label: "0-2 天", lo: 0, hi: 2},
		{label: "3-5 天", lo: 3, hi: 5},
		{label: "6-10 天", lo: 6, hi: 10},
		{label: "11-20 天", lo: 11, hi: 20},
		{label: "21-35 天", lo: 21, hi: 35},
		{label: "36+ 天", lo: 36, hi: 1 << 30},
	}
	for _, b := range bars {
		for _, bk := range buckets {
			if b.DaysToExpiry >= bk.lo && b.DaysToExpiry <= bk.hi {
				bk.sum += b.Basis
				bk.absSum += abs(b.Basis)
				bk.count++
				break
			}
		}
	}

	fmt.Println("=== Basis 收斂（依到期天數分桶）===")
	fmt.Printf("%-10s %8s %12s %12s\n", "區間", "天數", "平均Basis", "平均|Basis|")
	for _, bk := range buckets {
		if bk.count == 0 {
			continue
		}
		fmt.Printf("%-10s %8d %12.2f %12.2f\n", bk.label, bk.count, bk.sum/float64(bk.count), bk.absSum/float64(bk.count))
	}
	fmt.Println()
}

// printContango reports the overall contango/backwardation split and the
// per-year average basis, to surface any trend over time.
func printContango(bars []basis.Bar) {
	var contango, backwardation int
	var sum float64
	byYear := make(map[int][]float64)
	for _, b := range bars {
		if b.Basis > 0 {
			contango++
		} else if b.Basis < 0 {
			backwardation++
		}
		sum += b.Basis
		byYear[b.Date.Year()] = append(byYear[b.Date.Year()], b.Basis)
	}

	fmt.Println("=== Contango / Backwardation ===")
	fmt.Printf("整體平均 Basis: %.2f 點（%d 天，Contango(>0) %d 天 / Backwardation(<0) %d 天）\n",
		sum/float64(len(bars)), len(bars), contango, backwardation)

	years := make([]int, 0, len(byYear))
	for y := range byYear {
		years = append(years, y)
	}
	sort.Ints(years)
	fmt.Printf("%-6s %10s %8s\n", "年份", "平均Basis", "天數")
	for _, y := range years {
		vals := byYear[y]
		var s float64
		for _, v := range vals {
			s += v
		}
		fmt.Printf("%-6d %10.2f %8d\n", y, s/float64(len(vals)), len(vals))
	}
	fmt.Println()
}

// printRollingCost cross-references Phase 2's fixed-method roll dates
// against the specific freshly-rolled-into contract's Basis that day, as a
// direct, data-grounded measure of what continuously rolling a long
// position costs or earns. This deliberately does NOT reuse basis.Bar's
// per-day front-month (day.Months[0]): on the roll day itself, the old
// (about-to-expire tomorrow) contract is still nearest-by-settlement-date,
// so Months[0] would report the OLD contract's near-zero near-expiry basis,
// not the new contract's — the two "front months" only diverge on roll
// days, but that's exactly the day this analysis needs to get right.
func printRollingCost(days []contract.DaySnapshot, indexBars []parser.IndexBar) {
	daysByDate := make(map[string]contract.DaySnapshot, len(days))
	for _, d := range days {
		daysByDate[d.Date.Format("2006-01-02")] = d
	}
	indexByDate := make(map[string]float64, len(indexBars))
	for _, b := range indexBars {
		indexByDate[b.Date.Format("2006-01-02")] = b.PriceIndex
	}

	cbars, err := roll.Build(days, roll.NewFixedSelector(days))
	if err != nil {
		log.Fatalf("build fixed continuous series: %v", err)
	}

	var rollBasisSum float64
	var rollCount int
	for _, b := range cbars {
		if !b.RollDay {
			continue
		}
		key := b.Date.Format("2006-01-02")
		idx, ok := indexByDate[key]
		if !ok {
			continue // outside TAIEX coverage
		}
		day := daysByDate[key]
		month, ok := findMonth(day.Months, b.ContractMonth)
		if !ok {
			log.Fatalf("%s: cannot find contract %s in day snapshot", key, b.ContractMonth)
		}
		close, err := month.Bar.Close()
		if err != nil {
			log.Fatalf("%s: %v", key, err)
		}
		rollBasisSum += close - idx
		rollCount++
	}

	fmt.Println("=== Rolling Cost（每次轉倉當天，新契約的 Basis）===")
	if rollCount == 0 {
		fmt.Println("(沒有落在 TAIEX 資料涵蓋範圍內的轉倉)")
		return
	}
	fmt.Printf("涵蓋範圍內轉倉次數: %d\n", rollCount)
	fmt.Printf("平均每次轉倉 Basis: %.2f 點\n", rollBasisSum/float64(rollCount))
	fmt.Printf("累積轉倉 Basis 總和: %.2f 點\n", rollBasisSum)
}

func findMonth(months []contract.MonthSnapshot, cm string) (contract.MonthSnapshot, bool) {
	for _, m := range months {
		if m.ContractMonth == cm {
			return m, true
		}
	}
	return contract.MonthSnapshot{}, false
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
