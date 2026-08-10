package engine

import (
	"time"

	"strategy/internal/roll"
)

// DefaultInitialMargin and DefaultMaintenanceRatio are representative,
// explicitly-an-assumption placeholders: TAIFEX's actual TX original margin
// has moved roughly NT$120,000-260,000 over recent years as volatility
// changed, and we don't have that history, so a single fixed value is used
// for the whole backtest. 0.75 is the exchange's standard
// maintenance-to-original margin ratio.
const (
	DefaultInitialMargin    = 184000.0
	DefaultMaintenanceRatio = 0.75
)

// MarginCallEvent records a day the simulated account's equity fell below
// maintenance margin; the account is always topped back up to InitialMargin
// (never force-liquidated — see plan notes on why forced liquidation is out
// of scope before Phase 5 has re-entry logic).
type MarginCallEvent struct {
	Date         time.Time
	EquityBefore float64
	TopUp        float64
}

// Params configures one fund-mode (資金報酬) run.
type Params struct {
	Cost              CostModel
	StartingCapital   float64 // capital the trader allocates, NT$
	InitialMargin     float64 // NT$ required to open/restore the position (see ContractMultiplier)
	MaintenanceMargin float64 // NT$ floor that triggers a margin call

	// ContractMultiplier is how many contracts' worth of exposure and cost
	// the position carries (may be fractional — this is a research tool, not
	// a real order size). Zero defaults to 1 (Phase 3-5's fixed 1-contract
	// buy-and-hold). InitialMargin/MaintenanceMargin are not auto-scaled by
	// this — callers size them to match the actual position (e.g.
	// ContractMultiplier * per-contract margin) so the two stay orthogonal.
	ContractMultiplier float64
}

// FundResult is the simulated margin-account accounting of a buy-and-hold
// backtest.
type FundResult struct {
	Dates       []time.Time
	Equity      []float64 // actual simulated account balance, post any top-ups
	NetPnL      []float64 // cumulative P&L excluding top-ups (see RunFundMode)
	ReturnPct   []float64 // NetPnL / StartingCapital
	MarginCalls []MarginCallEvent

	Sharpe               float64
	MaxDrawdownPct       float64
	TotalCostNT          float64
	TotalInjectedCapital float64
}

// RunFundMode simulates holding 1 long contract from the first bar to the
// last, marking to market in NT$ against StartingCapital.
//
// NetPnL deliberately excludes margin-call top-ups: Equity is topped up to
// InitialMargin whenever it falls below MaintenanceMargin, but that
// injected cash is not portfolio growth, so Sharpe/MaxDrawdown are computed
// on NetPnL (equivalently: the P&L an account with unlimited pockets to
// meet margin calls would show), not on Equity. MarginCalls/
// TotalInjectedCapital separately report how much extra capital this
// leverage level actually required to survive.
func RunFundMode(bars []roll.ContinuousBar, p Params) FundResult {
	mult := p.ContractMultiplier
	if mult == 0 {
		mult = 1
	}

	n := len(bars)
	res := FundResult{
		Dates:     make([]time.Time, n),
		Equity:    make([]float64, n),
		NetPnL:    make([]float64, n),
		ReturnPct: make([]float64, n),
	}

	equity := p.StartingCapital
	var netPnL, injected float64
	dailyReturns := make([]float64, n)

	for i, b := range bars {
		res.Dates[i] = b.Date

		costNT := float64(costSides(bars, i)) * p.Cost.PerSideNT(*b.Close) * mult
		res.TotalCostNT += costNT

		var move float64
		if i > 0 {
			move = *bars[i].AdjustedClose - *bars[i-1].AdjustedClose
		}

		dayPnL := move*PointValue*mult - costNT
		equity += dayPnL
		netPnL += dayPnL

		if equity < p.MaintenanceMargin {
			topUp := p.InitialMargin - equity
			res.MarginCalls = append(res.MarginCalls, MarginCallEvent{
				Date:         b.Date,
				EquityBefore: equity,
				TopUp:        topUp,
			})
			equity += topUp
			injected += topUp
		}

		res.Equity[i] = equity
		res.NetPnL[i] = netPnL
		res.ReturnPct[i] = netPnL / p.StartingCapital
		dailyReturns[i] = dayPnL / p.StartingCapital
	}

	res.TotalInjectedCapital = injected
	res.Sharpe = sharpe(dailyReturns)
	res.MaxDrawdownPct = maxDrawdown(res.ReturnPct)
	return res
}
