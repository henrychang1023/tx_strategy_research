package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// LoadFutureCSV reads one year of TAIFEX all-products daily futures data and
// returns only rows for the given contract (e.g. "TX"), excluding cross-month
// spread rows (contract month like "202001/202002"). Both Regular and
// After-Hours session rows are kept; session filtering is a downstream
// (roll/cleaner) concern.
func LoadFutureCSV(path, contract string) ([]FutureBar, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // yearly files disagree on trailing empty column count

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("%s: reading header: %w", path, err)
	}
	col, err := columnIndex(header)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var bars []FutureBar
	for line := 2; ; line++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}

		if get(rec, col, "contract") != contract {
			continue
		}
		month := strings.TrimSpace(get(rec, col, "contract month(Week)"))
		if strings.Contains(month, "/") {
			continue // cross-month spread row, not a single contract
		}

		bar, err := parseFutureRow(rec, col, month)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		bars = append(bars, bar)
	}
	return bars, nil
}

// LoadFutureYears loads and concatenates multiple yearly CSVs (see LoadFutureCSV).
func LoadFutureYears(paths []string, contract string) ([]FutureBar, error) {
	var all []FutureBar
	for _, path := range paths {
		bars, err := LoadFutureCSV(path, contract)
		if err != nil {
			return nil, err
		}
		all = append(all, bars...)
	}
	return all, nil
}

func parseFutureRow(rec []string, col map[string]int, month string) (FutureBar, error) {
	var bar FutureBar
	var err error

	if bar.Date, err = parseDate(get(rec, col, "date")); err != nil {
		return bar, err
	}
	bar.Contract = strings.TrimSpace(get(rec, col, "contract"))
	bar.ContractMonth = month

	fields := []struct {
		name string
		dst  **float64
	}{
		{"open", &bar.Open},
		{"high", &bar.High},
		{"low", &bar.Low},
		{"last", &bar.Last},
		{"Change", &bar.Change},
		{"historical_high", &bar.HistoricalHigh},
		{"historical_low", &bar.HistoricalLow},
	}
	for _, f := range fields {
		if *f.dst, err = parseOptFloat(get(rec, col, f.name)); err != nil {
			return bar, fmt.Errorf("%s: %w", f.name, err)
		}
	}

	if bar.ChangePercent, err = parseOptPercent(get(rec, col, "%")); err != nil {
		return bar, fmt.Errorf("%%: %w", err)
	}
	if bar.Volume, err = parseIntStrict(get(rec, col, "Volume")); err != nil {
		return bar, fmt.Errorf("Volume: %w", err)
	}
	if bar.SettlementPrice, err = parseOptFloat(get(rec, col, "settlement_price")); err != nil {
		return bar, fmt.Errorf("settlement_price: %w", err)
	}
	if bar.OpenInterest, err = parseOptInt(get(rec, col, "open_interest")); err != nil {
		return bar, fmt.Errorf("open_interest: %w", err)
	}
	if bar.BestBid, err = parseOptFloat(get(rec, col, "best_bid")); err != nil {
		return bar, fmt.Errorf("best_bid: %w", err)
	}
	if bar.BestAsk, err = parseOptFloat(get(rec, col, "best_ask")); err != nil {
		return bar, fmt.Errorf("best_ask: %w", err)
	}
	bar.TradingSession = strings.TrimSpace(get(rec, col, "Trading Session"))

	return bar, nil
}

// columnIndex maps trimmed header names to their column index.
func columnIndex(header []string) (map[string]int, error) {
	col := make(map[string]int, len(header))
	for i, name := range header {
		col[strings.TrimSpace(name)] = i
	}
	required := []string{
		"date", "contract", "contract month(Week)", "open", "high", "low", "last",
		"Change", "%", "Volume", "settlement_price", "open_interest",
		"best_bid", "best_ask", "historical_high", "historical_low", "Trading Session",
	}
	for _, name := range required {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("missing expected column %q", name)
		}
	}
	return col, nil
}

// get returns the trimmed value of column name in rec, or "" if the row is
// too short (ragged trailing columns) or the column is unknown.
func get(rec []string, col map[string]int, name string) string {
	i, ok := col[name]
	if !ok || i >= len(rec) {
		return ""
	}
	return rec[i]
}
