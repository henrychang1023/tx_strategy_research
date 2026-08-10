package cleaner

import (
	"encoding/csv"
	"os"
	"strconv"

	"strategy/internal/parser"
)

const dateLayout = "2006-01-02"

var futureHeader = []string{
	"date", "contract", "contract_month", "session",
	"open", "high", "low", "last", "change", "change_percent",
	"volume", "settlement_price", "open_interest", "best_bid", "best_ask",
	"historical_high", "historical_low",
}

// WriteFutureCSV writes bars (expected to already be sorted, see SortFuture)
// as a clean CSV with a unified date format and blank cells for missing
// values, replacing any file already at path.
func WriteFutureCSV(path string, bars []parser.FutureBar) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(futureHeader); err != nil {
		return err
	}
	for _, b := range bars {
		row := []string{
			b.Date.Format(dateLayout),
			b.Contract,
			b.ContractMonth,
			b.TradingSession,
			floatStr(b.Open),
			floatStr(b.High),
			floatStr(b.Low),
			floatStr(b.Last),
			floatStr(b.Change),
			floatStr(b.ChangePercent),
			strconv.FormatInt(b.Volume, 10),
			floatStr(b.SettlementPrice),
			intStr(b.OpenInterest),
			floatStr(b.BestBid),
			floatStr(b.BestAsk),
			floatStr(b.HistoricalHigh),
			floatStr(b.HistoricalLow),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

var indexHeader = []string{"date", "price_index", "total_return_index", "change", "change_percent"}

// WriteIndexCSV writes bars (expected to already be sorted, see SortIndex)
// as a clean CSV with a unified date format, replacing any file already at path.
func WriteIndexCSV(path string, bars []parser.IndexBar) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(indexHeader); err != nil {
		return err
	}
	for _, b := range bars {
		row := []string{
			b.Date.Format(dateLayout),
			strconv.FormatFloat(b.PriceIndex, 'f', -1, 64),
			strconv.FormatFloat(b.TotalReturnIndex, 'f', -1, 64),
			strconv.FormatFloat(b.Change, 'f', -1, 64),
			strconv.FormatFloat(b.ChangePercent, 'f', -1, 64),
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
