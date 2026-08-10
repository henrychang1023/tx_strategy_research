package basis

import (
	"encoding/csv"
	"os"
	"strconv"
)

const dateLayout = "2006-01-02"

var header = []string{"date", "contract_month", "future", "index", "basis", "days_to_expiry"}

// WriteCSV writes bars (expected ascending by date, as returned by Compute)
// as a clean CSV, replacing any file already at path.
func WriteCSV(path string, bars []Bar) error {
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
			strconv.FormatFloat(b.Future, 'f', -1, 64),
			strconv.FormatFloat(b.Index, 'f', -1, 64),
			strconv.FormatFloat(b.Basis, 'f', -1, 64),
			strconv.Itoa(b.DaysToExpiry),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
