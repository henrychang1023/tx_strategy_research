// extra answers four follow-up research questions raised after Phase 4/5
// (see 額外問題.md):
//  1. Has contango actually eaten into returns, net of backwardation gains?
//  2. 2024 was the one year with average positive (contango) basis — how did
//     the seamless-roll continuous contract compare to TAIEX in that year alone?
//  3. What does the rolling cost/benefit look like broken down per calendar year?
//  4. What does the continuous (back-adjusted) price series itself look like,
//     and what does comparing it to the raw un-adjusted front-month series show?
package main

import (
	"fmt"
	"log"
	"math"
	"sort"

	"strategy/internal/cleaner"
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

const taiexFile = "data/raw/twse/Taiwan Stock Exchange Capitalization Weighted Stock Index.csv"

// rollBasis is one roll's freshly-rolled-into-contract basis, dated.
type rollBasis struct {
	date  string
	basis float64
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
	indexBars, err := parser.LoadTAIEXCSV(taiexFile)
	if err != nil {
		log.Fatalf("load %s: %v", taiexFile, err)
	}
	indexBars = cleaner.SortIndex(indexBars)
	indexByDate := make(map[string]parser.IndexBar, len(indexBars))
	for _, b := range indexBars {
		indexByDate[b.Date.Format("2006-01-02")] = b
	}

	cbars, err := roll.Build(days, roll.NewFixedSelector(days))
	if err != nil {
		log.Fatalf("build fixed continuous series: %v", err)
	}

	rolls := rollBases(cbars, indexByDate)

	q1ContangoVsBackwardation(rolls)
	q2Year2024(cbars, indexBars)
	q3RollingCostPerYear(rolls)
	q4ContinuousPriceObservations(cbars)
}

// rollBases computes, for every roll within TAIEX's coverage, the freshly
// rolled-into contract's Basis (raw Close - Index) that same day. cbars'
// Close on a RollDay is already the new contract's raw price (that's what
// the selector chose), so no per-contract lookup is needed here — unlike
// Phase 4's basis.Compute (which reports whichever contract is nearest by
// settlement date, still the OLD one on the roll day itself).
func rollBases(cbars []roll.ContinuousBar, indexByDate map[string]parser.IndexBar) []rollBasis {
	var out []rollBasis
	for _, b := range cbars {
		if !b.RollDay {
			continue
		}
		key := b.Date.Format("2006-01-02")
		idx, ok := indexByDate[key]
		if !ok {
			continue // outside TAIEX coverage
		}
		out = append(out, rollBasis{date: key, basis: *b.Close - idx.PriceIndex})
	}
	return out
}

// q1ContangoVsBackwardation splits at-roll basis by sign: a positive basis
// (contango) means the freshly-rolled-into contract was bought at a premium
// to the index, which is a cost as that premium converges away; a negative
// basis (backwardation) is a discount that converges into a gain.
func q1ContangoVsBackwardation(rolls []rollBasis) {
	var contangoSum, backwardationSum float64
	var contangoN, backwardationN int
	for _, r := range rolls {
		if r.basis > 0 {
			contangoSum += r.basis
			contangoN++
		} else if r.basis < 0 {
			backwardationSum += r.basis
			backwardationN++
		}
	}
	net := contangoSum + backwardationSum

	fmt.Println("=== Q1: Contango 有沒有吃掉報酬 ===")
	fmt.Printf("Contango 轉倉（正 Basis，對多單是成本）：%d 次，合計 %+.2f 點\n", contangoN, contangoSum)
	fmt.Printf("Backwardation 轉倉（負 Basis，對多單是利得）：%d 次，合計 %+.2f 點\n", backwardationN, backwardationSum)
	fmt.Printf("淨合計：%+.2f 點\n\n", net)
}

// q2Year2024 compares TX's continuous contract against TAIEX within 2024
// only — the one year Phase 4 found average basis was positive (contango).
func q2Year2024(cbars []roll.ContinuousBar, indexBars []parser.IndexBar) {
	var firstTX, lastTX *roll.ContinuousBar
	for i := range cbars {
		if cbars[i].Date.Year() != 2024 {
			continue
		}
		if firstTX == nil {
			firstTX = &cbars[i]
		}
		lastTX = &cbars[i]
	}

	var firstIdx, lastIdx *parser.IndexBar
	for i := range indexBars {
		if indexBars[i].Date.Year() != 2024 {
			continue
		}
		if firstIdx == nil {
			firstIdx = &indexBars[i]
		}
		lastIdx = &indexBars[i]
	}

	if firstTX == nil || firstIdx == nil {
		log.Fatal("no 2024 data found")
	}

	txPoints := *lastTX.AdjustedClose - *firstTX.AdjustedClose
	txPct := txPoints / *firstTX.Close * 100 // % return uses the real (raw) starting price, not the synthetic adjusted one

	priPct := (lastIdx.PriceIndex - firstIdx.PriceIndex) / firstIdx.PriceIndex * 100
	triPct := (lastIdx.TotalReturnIndex - firstIdx.TotalReturnIndex) / firstIdx.TotalReturnIndex * 100

	fmt.Println("=== Q2: 2024 年單獨比較（2024 是唯一平均 Basis 為正的年份）===")
	fmt.Printf("TX 連續契約（fixed）：%s(%.0f) -> %s(%.0f)，%+.1f 點，%+.2f%%\n",
		firstTX.Date.Format("2006-01-02"), *firstTX.Close, lastTX.Date.Format("2006-01-02"), *lastTX.Close, txPoints, txPct)
	fmt.Printf("TAIEX Price Index：%+.2f%%\n", priPct)
	fmt.Printf("TAIEX Total Return Index：%+.2f%%\n\n", triPct)
}

func q3RollingCostPerYear(rolls []rollBasis) {
	byYear := make(map[string]float64)
	years := make([]string, 0)
	for _, r := range rolls {
		year := r.date[:4]
		if _, ok := byYear[year]; !ok {
			years = append(years, year)
		}
		byYear[year] += r.basis
	}
	sort.Strings(years)

	fmt.Println("=== Q3: Rolling Cost 逐年拆解（新契約轉倉當下的 Basis 加總）===")
	fmt.Printf("%-6s %12s\n", "年份", "合計(點)")
	for _, y := range years {
		fmt.Printf("%-6s %12.2f\n", y, byYear[y])
	}
	fmt.Println()
}

func q4ContinuousPriceObservations(cbars []roll.ContinuousBar) {
	fmt.Println("=== Q4: Continuous Price 觀察 ===")

	// A concrete worked example of the back-adjustment mechanic, using a
	// real roll from the data instead of illustrative numbers.
	for i, b := range cbars {
		if !b.RollDay || i == 0 {
			continue
		}
		prev := cbars[i-1]
		fmt.Printf("實例：%s %s 收盤 %.0f -> %s %s 收盤 %.0f，跳空 %+.0f 點；\n",
			prev.Date.Format("2006-01-02"), prev.ContractMonth, *prev.Close,
			b.Date.Format("2006-01-02"), b.ContractMonth, *b.Close, *b.Close-*prev.Close)
		fmt.Printf("      back-adjust 後，同一天 adjusted close 變動為 %+.0f 點（= 舊契約當天自己的真實漲跌，不含跳空）\n\n",
			*b.AdjustedClose-*prev.AdjustedClose)
		break // just need one representative example
	}

	rawCloses := make([]float64, len(cbars))
	adjCloses := make([]float64, len(cbars))
	for i, b := range cbars {
		rawCloses[i] = *b.Close
		adjCloses[i] = *b.AdjustedClose
	}

	fmt.Printf("累積 back-adjust 位移量（起點 raw - adjusted）：%.1f 點\n",
		rawCloses[0]-adjCloses[0])
	fmt.Printf("原始前月序列 MaxDrawdown：%.1f 點\n", maxDrawdown(rawCloses))
	fmt.Printf("連續契約序列 MaxDrawdown：%.1f 點（未扣成本，Phase 3 報告的 7478.7 點另外扣了進出場/轉倉成本，量級一致）\n", maxDrawdown(adjCloses))
	fmt.Printf("原始前月序列 Sharpe（逐日差含轉倉跳空）：%.2f\n", sharpe(diffs(rawCloses)))
	fmt.Printf("連續契約序列 Sharpe：%.2f（未扣成本，Phase 3 報告的 0.94 另外扣了成本，量級一致）\n", sharpe(diffs(adjCloses)))
}

func diffs(series []float64) []float64 {
	out := make([]float64, 0, len(series)-1)
	for i := 1; i < len(series); i++ {
		out = append(out, series[i]-series[i-1])
	}
	return out
}

func maxDrawdown(series []float64) float64 {
	if len(series) == 0 {
		return 0
	}
	peak := series[0]
	var maxDD float64
	for _, v := range series {
		if v > peak {
			peak = v
		}
		if dd := peak - v; dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}

func sharpe(dailyReturns []float64) float64 {
	n := len(dailyReturns)
	if n < 2 {
		return 0
	}
	var sum float64
	for _, r := range dailyReturns {
		sum += r
	}
	mean := sum / float64(n)
	var sumSq float64
	for _, r := range dailyReturns {
		d := r - mean
		sumSq += d * d
	}
	stdev := math.Sqrt(sumSq / float64(n-1))
	if stdev == 0 {
		return 0
	}
	return mean / stdev * math.Sqrt(252)
}
