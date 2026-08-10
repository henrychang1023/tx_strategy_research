package roll

import (
	"encoding/csv"
	"os"
	"strconv"
)

const dateLayout = "2006-01-02"

var header = []string{
	"date", "contract_month", "roll_day",
	"open", "high", "low", "close",
	"adjusted_open", "adjusted_high", "adjusted_low", "adjusted_close",
	"volume", "open_interest",
}

// WriteCSV writes bars (expected to already be in ascending date order, as
// returned by Build) as a clean CSV, replacing any file already at path.
func WriteCSV(path string, bars []ContinuousBar) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		return err
	}
	for _, b := range bars {
		row := []string{
			b.Date.Format(dateLayout),
			b.ContractMonth,
			strconv.FormatBool(b.RollDay),
			floatStr(b.Open),
			floatStr(b.High),
			floatStr(b.Low),
			floatStr(b.Close),
			floatStr(b.AdjustedOpen),
			floatStr(b.AdjustedHigh),
			floatStr(b.AdjustedLow),
			floatStr(b.AdjustedClose),
			strconv.FormatInt(b.Volume, 10),
			intStr(b.OpenInterest),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func floatStr(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}

func intStr(p *int64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(*p, 10)
}
