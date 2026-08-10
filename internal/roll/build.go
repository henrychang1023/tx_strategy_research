package roll

import (
	"fmt"

	"strategy/internal/contract"
	"strategy/internal/parser"
)

// Build walks days in order, uses sel to pick each day's front contract, and
// back-adjusts the resulting series so roll days don't show artificial price
// gaps.
func Build(days []contract.DaySnapshot, sel Selector) ([]ContinuousBar, error) {
	bars := make([]ContinuousBar, 0, len(days))
	prevContractMonth := ""

	for _, day := range days {
		cm, err := sel.Next(day)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", day.Date.Format("2006-01-02"), err)
		}
		month, ok := findMonth(day.Months, cm)
		if !ok {
			return nil, fmt.Errorf("%s: selector chose unknown contract month %q", day.Date.Format("2006-01-02"), cm)
		}
		close, err := closePrice(month.Bar)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", day.Date.Format("2006-01-02"), err)
		}

		bars = append(bars, ContinuousBar{
			Date:          day.Date,
			ContractMonth: cm,
			RollDay:       prevContractMonth != "" && cm != prevContractMonth,
			Open:          month.Bar.Open,
			High:          month.Bar.High,
			Low:           month.Bar.Low,
			Close:         close,
			Volume:        month.Bar.Volume,
			OpenInterest:  month.Bar.OpenInterest,
		})
		prevContractMonth = cm
	}

	if err := backAdjust(bars, days); err != nil {
		return nil, err
	}
	return bars, nil
}

// backAdjust computes, per ContinuousBar, an Adjusted* price such that the
// most recent contract segment is left at its raw price and every earlier
// segment is shifted by the cumulative sum of roll-day gaps. A roll-day gap
// is (old contract's close - new contract's close) both evaluated on the
// roll day itself, so the day-over-day change across the roll reflects the
// old contract's own move rather than the cross-contract price difference.
func backAdjust(bars []ContinuousBar, days []contract.DaySnapshot) error {
	byDate := make(map[string]contract.DaySnapshot, len(days))
	for _, d := range days {
		byDate[d.Date.Format("2006-01-02")] = d
	}

	var cumulative float64
	for i := len(bars) - 1; i >= 0; i-- {
		b := &bars[i]
		b.AdjustedOpen = adjustPtr(b.Open, cumulative)
		b.AdjustedHigh = adjustPtr(b.High, cumulative)
		b.AdjustedLow = adjustPtr(b.Low, cumulative)
		b.AdjustedClose = adjustPtr(b.Close, cumulative)

		if !b.RollDay {
			continue
		}
		if i == 0 {
			return fmt.Errorf("%s: marked as roll day but has no prior bar", b.Date.Format("2006-01-02"))
		}
		prevCM := bars[i-1].ContractMonth
		day := byDate[b.Date.Format("2006-01-02")]
		oldMonth, ok := findMonth(day.Months, prevCM)
		if !ok {
			return fmt.Errorf("%s: cannot find prior contract %s for back-adjustment", b.Date.Format("2006-01-02"), prevCM)
		}
		oldClose, err := closePrice(oldMonth.Bar)
		if err != nil {
			return fmt.Errorf("%s: %w", b.Date.Format("2006-01-02"), err)
		}
		if b.Close == nil {
			return fmt.Errorf("%s: missing close price for back-adjustment gap", b.Date.Format("2006-01-02"))
		}
		cumulative += *oldClose - *b.Close
	}
	return nil
}

func closePrice(b parser.FutureBar) (*float64, error) {
	v, err := b.Close()
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func findMonth(months []contract.MonthSnapshot, cm string) (contract.MonthSnapshot, bool) {
	for _, m := range months {
		if m.ContractMonth == cm {
			return m, true
		}
	}
	return contract.MonthSnapshot{}, false
}

func adjustPtr(p *float64, offset float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p - offset
	return &v
}
