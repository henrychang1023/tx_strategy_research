package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseDate handles both zero-padded ("2021/01/04") and non-padded
// ("2020/1/2") date formats found across yearly source files.
func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006/1/2", "2006/01/02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q", s)
}

// parseFloat parses a required numeric field.
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

// parseOptFloat parses a numeric field that may be "-" (missing).
func parseOptFloat(s string) (*float64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// parseIntStrict parses a required integer field.
func parseIntStrict(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

// parseOptInt parses an integer field that may be "-" (missing).
func parseOptInt(s string) (*int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return nil, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// parsePercent parses a required trailing-"%" field like "0.90%" or "-0.13%" into 0.90 / -0.13.
func parsePercent(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	return strconv.ParseFloat(s, 64)
}

// parseOptPercent parses a trailing-"%" field that may be "-" (missing).
func parseOptPercent(s string) (*float64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return nil, nil
	}
	return parseOptFloat(strings.TrimSuffix(s, "%"))
}
