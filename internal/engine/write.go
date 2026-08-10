package engine

import (
	"encoding/csv"
	"os"
	"strconv"
)

const dateLayout = "2006-01-02"

var pointHeader = []string{"date", "daily_points", "cumulative_points"}

// WritePointCSV writes a PointResult, replacing any file already at path.
func WritePointCSV(path string, res PointResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(pointHeader); err != nil {
		return err
	}
	for i, d := range res.Dates {
		row := []string{
			d.Format(dateLayout),
			strconv.FormatFloat(res.DailyPoints[i], 'f', -1, 64),
			strconv.FormatFloat(res.CumulativePoints[i], 'f', -1, 64),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// NamedFundResult pairs a FundResult with a label (e.g. leverage scenario
// name) for WriteFundCSV.
type NamedFundResult struct {
	Scenario string
	Result   FundResult
}

var fundHeader = []string{"date", "scenario", "equity", "net_pnl", "return_pct"}

// WriteFundCSV writes one or more scenarios' FundResults into a single CSV
// distinguished by the scenario column, replacing any file already at path.
func WriteFundCSV(path string, scenarios []NamedFundResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(fundHeader); err != nil {
		return err
	}
	for _, s := range scenarios {
		r := s.Result
		for i, d := range r.Dates {
			row := []string{
				d.Format(dateLayout),
				s.Scenario,
				strconv.FormatFloat(r.Equity[i], 'f', -1, 64),
				strconv.FormatFloat(r.NetPnL[i], 'f', -1, 64),
				strconv.FormatFloat(r.ReturnPct[i], 'f', -1, 64),
			}
			if err := w.Write(row); err != nil {
				return err
			}
		}
	}
	w.Flush()
	return w.Error()
}
