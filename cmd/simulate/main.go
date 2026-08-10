// simulate answers the final research question (見 模擬策略.md): calibrate
// the fund-mode leverage (StartingCapital) of the fixed-roll-method
// buy-and-hold strategy so its daily-return volatility matches TAIEX Total
// Return Index's, then compares the two total returns over the same
// 2021-2025 window.
//
// Key reuse: engine.RunFundMode's NetPnL is, by construction (see
// internal/engine/fund.go), independent of StartingCapital/InitialMargin —
// margin-call top-ups never touch it. So calling it once with a placeholder
// StartingCapital=1 yields the true leverage-independent daily $ P&L series,
// which is all that's needed to solve for the volatility-matching capital
// analytically (no search/iteration needed): the daily %-return stdev of a
// fixed 1-contract position scales as stdev($PnL)/StartingCapital, so
// StartingCapital = stdev($PnL) / target %-stdev matches it in one step.
package main

import (
	"fmt"
	"log"
	"math"
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

const taiexFile = "data/raw/twse/Taiwan Stock Exchange Capitalization Weighted Stock Index.csv"

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
	indexByDate := make(map[time.Time]parser.IndexBar, len(indexBars))
	for _, b := range indexBars {
		indexByDate[b.Date] = b
	}

	// Restrict TX to TAIEX's coverage window (2021-01-04 onward, see Phase 1
	// notes) so both sides compare the same trading days.
	window := trimToTAIEXWindow(cbars, indexBars[0].Date, indexBars[len(indexBars)-1].Date)
	fmt.Printf("比較區間: %s -> %s（%d 個交易日，兩邊日曆完全對齊，見 Phase 4）\n\n",
		window[0].Date.Format("2006-01-02"), window[len(window)-1].Date.Format("2006-01-02"), len(window))

	cost := engine.DefaultCostModel

	// Step 1: leverage-independent $ P&L series (see package doc comment).
	placeholder := engine.RunFundMode(window, engine.Params{
		Cost: cost, StartingCapital: 1, InitialMargin: 1, MaintenanceMargin: 1,
	})
	dollarStdev := stdev(dailySeries(placeholder.NetPnL))

	// Step 2: TAIEX Total Return Index daily % returns and level series
	// over the same window.
	triLevels := make([]float64, len(window))
	triReturns := make([]float64, 0, len(window)-1)
	for i, b := range window {
		triLevels[i] = indexByDate[b.Date].TotalReturnIndex
		if i > 0 {
			triReturns = append(triReturns, (triLevels[i]-triLevels[i-1])/triLevels[i-1])
		}
	}
	taiexStdev := stdev(triReturns)

	// Step 3: solve for the volatility-matching StartingCapital.
	startingCapital := dollarStdev / taiexStdev
	fmt.Printf("TX 策略每日台幣損益標準差: NT$%.0f\n", dollarStdev)
	fmt.Printf("TAIEX 累積報酬指數每日報酬標準差: %.4f%%\n", taiexStdev*100)
	fmt.Printf("=> 風險相近所需本金: NT$%.0f\n\n", startingCapital)

	initialMargin := engine.DefaultInitialMargin
	maintenanceMargin := initialMargin * engine.DefaultMaintenanceRatio
	if startingCapital < initialMargin {
		fmt.Printf("警告: 這個本金 (NT$%.0f) 低於原始保證金 (NT$%.0f)，實務上開不了倉，後續數字僅供參考。\n",
			startingCapital, initialMargin)
	}
	firstNotional := *window[0].Close * engine.PointValue
	fmt.Printf("起始名目價值: NT$%.0f，隱含起始槓桿倍數約 %.2fx\n\n", firstNotional, firstNotional/startingCapital)

	// Step 4: run the real simulation at the calibrated capital.
	result := engine.RunFundMode(window, engine.Params{
		Cost: cost, StartingCapital: startingCapital,
		InitialMargin: initialMargin, MaintenanceMargin: maintenanceMargin,
	})
	actualStdev := stdev(dailySeries(result.NetPnL)) / startingCapital

	txReturn := result.ReturnPct[len(result.ReturnPct)-1] * 100
	txMaxDD := result.MaxDrawdownPct * 100

	taiexReturn := (triLevels[len(triLevels)-1] - triLevels[0]) / triLevels[0] * 100
	taiexMaxDD := maxDrawdown(triLevels) / triLevels[0] * 100 // as % of the starting level, comparable to MaxDrawdownPct
	taiexSharpe := sharpe(triReturns)

	fmt.Println("=== 校準結果驗證（兩邊年化波動度應該非常接近）===")
	fmt.Printf("TX 策略年化波動度:   %.2f%%\n", actualStdev*math.Sqrt(252)*100)
	fmt.Printf("TAIEX 年化波動度:    %.2f%%\n\n", taiexStdev*math.Sqrt(252)*100)

	fmt.Println("=== 風險相近後的報酬率比較 ===")
	fmt.Printf("%-24s %12s %10s %14s %10s\n", "標的", "總報酬%", "Sharpe", "最大回撤%", "追繳次數")
	fmt.Printf("%-24s %12.1f %10.2f %14.1f %10d\n", "TX 轉倉策略(校準槓桿)", txReturn, result.Sharpe, txMaxDD, len(result.MarginCalls))
	fmt.Printf("%-24s %12.1f %10.2f %14.1f %10s\n", "TAIEX Total Return", taiexReturn, taiexSharpe, taiexMaxDD, "-")
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

func dailySeries(cumulative []float64) []float64 {
	out := make([]float64, len(cumulative))
	prev := 0.0
	for i, v := range cumulative {
		out[i] = v - prev
		prev = v
	}
	return out
}

func stdev(xs []float64) float64 {
	n := len(xs)
	if n < 2 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(n)
	var sumSq float64
	for _, x := range xs {
		d := x - mean
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(n-1))
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
	sd := stdev(dailyReturns)
	if sd == 0 {
		return 0
	}
	return mean / sd * math.Sqrt(252)
}

// maxDrawdown returns the largest peak-to-trough decline in level (a raw
// index-level series), as a positive magnitude in the same unit as level.
func maxDrawdown(level []float64) float64 {
	if len(level) == 0 {
		return 0
	}
	peak := level[0]
	var maxDD float64
	for _, v := range level {
		if v > peak {
			peak = v
		}
		if dd := peak - v; dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}
