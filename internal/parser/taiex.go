package parser

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// utf8BOM is the 3-byte UTF-8 byte order mark some source files are prefixed with.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// LoadTAIEXCSV reads TWSE Capitalization Weighted Stock Index daily data.
func LoadTAIEXCSV(path string) ([]IndexBar, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw = bytes.TrimPrefix(raw, utf8BOM)

	r := csv.NewReader(bytes.NewReader(raw))

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("%s: reading header: %w", path, err)
	}
	col := make(map[string]int, len(header))
	for i, name := range header {
		col[strings.TrimSpace(name)] = i
	}
	for _, name := range []string{"Date", "Price Index", "Total Return Index", "Change", "%Change"} {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("%s: missing expected column %q", path, name)
		}
	}

	var bars []IndexBar
	for line := 2; ; line++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}

		var bar IndexBar
		if bar.Date, err = parseDate(get(rec, col, "Date")); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		if bar.PriceIndex, err = parseFloat(get(rec, col, "Price Index")); err != nil {
			return nil, fmt.Errorf("%s:%d: Price Index: %w", path, line, err)
		}
		if bar.TotalReturnIndex, err = parseFloat(get(rec, col, "Total Return Index")); err != nil {
			return nil, fmt.Errorf("%s:%d: Total Return Index: %w", path, line, err)
		}
		if bar.Change, err = parseFloat(get(rec, col, "Change")); err != nil {
			return nil, fmt.Errorf("%s:%d: Change: %w", path, line, err)
		}
		if bar.ChangePercent, err = parsePercent(get(rec, col, "%Change")); err != nil {
			return nil, fmt.Errorf("%s:%d: %%Change: %w", path, line, err)
		}
		bars = append(bars, bar)
	}
	return bars, nil
}
