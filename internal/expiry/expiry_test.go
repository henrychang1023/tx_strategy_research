package expiry

import (
	"testing"
	"time"
)

func TestSettlementDate(t *testing.T) {
	cases := []struct {
		contractMonth string
		want          string
	}{
		{"202001", "2020-01-15"}, // month starts on a Wednesday
		{"202106", "2021-06-16"}, // month starts on a Tuesday
		{"202212", "2022-12-21"}, // month starts on a Thursday
		{"202303", "2023-03-15"}, // month starts on a Wednesday
		{"202409", "2024-09-18"}, // month starts on a Sunday
		{"202511", "2025-11-19"}, // month starts on a Saturday
		{"202605", "2026-05-20"}, // cross-checked against data/raw/expiry/expiry.csv
	}
	for _, c := range cases {
		got, err := SettlementDate(c.contractMonth)
		if err != nil {
			t.Fatalf("SettlementDate(%q): %v", c.contractMonth, err)
		}
		if got.Weekday() != time.Wednesday {
			t.Errorf("SettlementDate(%q) = %s, not a Wednesday", c.contractMonth, got.Format("2006-01-02"))
		}
		if got.Format("2006-01-02") != c.want {
			t.Errorf("SettlementDate(%q) = %s, want %s", c.contractMonth, got.Format("2006-01-02"), c.want)
		}
	}
}

func TestSettlementDateInvalid(t *testing.T) {
	for _, bad := range []string{"", "2020", "2020013", "2020ab"} {
		if _, err := SettlementDate(bad); err == nil {
			t.Errorf("SettlementDate(%q): expected error, got nil", bad)
		}
	}
}
