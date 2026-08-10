// Package engine runs the Phase 3 buy-and-hold backtest over a continuous
// contract series (internal/roll), in two accounting modes: pure points
// (comparable to TAIEX point returns) and simulated margin/fund returns
// (leverage-sensitive). See 台指期無縫轉倉研究計畫.md Phase 3 and
// C:\Users\User\.claude\plans\bright-watching-cerf.md for the design.
package engine

// PointValue is TX's fixed exchange-defined contract multiplier: NT$200 per
// index point per contract. This is not an assumption.
const PointValue = 200.0

// CostModel is the per-side (one buy or one sell) trading cost assumption.
// These are user-adjustable placeholders, not regulatory constants, except
// TaxRate which is Taiwan's current TX futures transaction tax rate.
type CostModel struct {
	CommissionPerContract float64 // broker commission per side, NT$
	TaxRate               float64 // transaction tax per side, fraction of notional (price * PointValue)
	SlippagePoints        float64 // assumed slippage per side, index points
}

// DefaultCostModel is a representative, clearly-an-assumption starting point:
// NT$50 commission per side, Taiwan's statutory 0.00002 TX transaction tax
// rate, and 1 point of slippage per side on a liquid front-month contract.
var DefaultCostModel = CostModel{
	CommissionPerContract: 50,
	TaxRate:               0.00002,
	SlippagePoints:        1,
}

// PerSidePoints converts all three cost components to index points for a
// trade executed at price.
func (c CostModel) PerSidePoints(price float64) float64 {
	return c.SlippagePoints + c.CommissionPerContract/PointValue + price*c.TaxRate
}

// PerSideNT converts all three cost components to NT$ for a trade executed
// at price.
func (c CostModel) PerSideNT(price float64) float64 {
	return c.PerSidePoints(price) * PointValue
}
