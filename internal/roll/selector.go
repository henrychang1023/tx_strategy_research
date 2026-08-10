package roll

import (
	"fmt"
	"sort"
	"time"

	"strategy/internal/contract"
)

// computeCutoffs maps each contract month to the last trading day it is
// still eligible to be selected as front. That day is the trading day
// immediately before the contract's settlement date, so a contract is
// always rolled out one trading day before it would settle (and its
// unreliable final-settlement-day price, see 台指期無縫轉倉研究計畫.md, is
// never used by the front selectors below). Months whose settlement date
// falls outside the observed trading calendar (e.g. far-dated contracts
// that haven't matured by the end of the dataset) get no entry, meaning
// they are never force-rolled.
func computeCutoffs(days []contract.DaySnapshot) map[string]time.Time {
	calendar := make([]time.Time, len(days))
	settleOf := make(map[string]time.Time)
	for i, d := range days {
		calendar[i] = d.Date
		for _, m := range d.Months {
			settleOf[m.ContractMonth] = m.SettlementDate
		}
	}

	cutoff := make(map[string]time.Time, len(settleOf))
	for cm, settle := range settleOf {
		idx := sort.Search(len(calendar), func(i int) bool { return !calendar[i].Before(settle) })
		if idx <= 0 || idx >= len(calendar) || !calendar[idx].Equal(settle) {
			continue // settlement date not found in calendar; leave unconstrained
		}
		cutoff[cm] = calendar[idx-1]
	}
	return cutoff
}

// fixedSelector always picks the nearest-to-expiry contract, rolled out one
// trading day before its settlement date.
type fixedSelector struct {
	cutoff map[string]time.Time
}

// NewFixedSelector implements 固定結算日前一天轉倉: front is always the
// nearest-to-expiry contract, switching to the next one on the trading day
// before the current front's settlement date.
func NewFixedSelector(days []contract.DaySnapshot) Selector {
	return &fixedSelector{cutoff: computeCutoffs(days)}
}

func (s *fixedSelector) Next(day contract.DaySnapshot) (string, error) {
	for _, m := range day.Months { // ascending by settlement date
		if cut, ok := s.cutoff[m.ContractMonth]; ok && !day.Date.Before(cut) {
			continue // rolled out as of today
		}
		return m.ContractMonth, nil
	}
	return "", fmt.Errorf("no eligible contract on %s", day.Date.Format("2006-01-02"))
}

// metricSelector implements a sticky, one-directional switch: it stays on
// the current front contract until the next-nearest contract's metric
// (volume or open interest) exceeds it, or the cutoff forces a roll.
type metricSelector struct {
	cutoff  map[string]time.Time
	metric  func(contract.MonthSnapshot) (float64, bool)
	current string
}

// NewVolumeSelector implements Volume 切換: rolls to the next contract once
// its Regular-session Volume exceeds the current front's.
func NewVolumeSelector(days []contract.DaySnapshot) Selector {
	return &metricSelector{
		cutoff: computeCutoffs(days),
		metric: func(m contract.MonthSnapshot) (float64, bool) { return float64(m.Bar.Volume), true },
	}
}

// NewOISelector implements Open Interest 切換: rolls to the next contract
// once its Regular-session OpenInterest exceeds the current front's.
func NewOISelector(days []contract.DaySnapshot) Selector {
	return &metricSelector{
		cutoff: computeCutoffs(days),
		metric: func(m contract.MonthSnapshot) (float64, bool) {
			if m.Bar.OpenInterest == nil {
				return 0, false
			}
			return float64(*m.Bar.OpenInterest), true
		},
	}
}

func (s *metricSelector) Next(day contract.DaySnapshot) (string, error) {
	if len(day.Months) == 0 {
		return "", fmt.Errorf("no contracts available on %s", day.Date.Format("2006-01-02"))
	}

	idx := -1
	for i, m := range day.Months {
		if m.ContractMonth == s.current {
			idx = i
			break
		}
	}
	if idx == -1 {
		// first day, or the previous front is unexpectedly absent from
		// today's snapshot: (re)initialize to the nearest contract.
		s.current = day.Months[0].ContractMonth
		return s.current, nil
	}

	front := day.Months[idx]
	if cut, ok := s.cutoff[front.ContractMonth]; ok && !day.Date.Before(cut) {
		if idx+1 >= len(day.Months) {
			return "", fmt.Errorf("%s: contract %s hit its roll cutoff with no next contract available",
				day.Date.Format("2006-01-02"), front.ContractMonth)
		}
		s.current = day.Months[idx+1].ContractMonth
		return s.current, nil
	}

	if idx+1 < len(day.Months) {
		next := day.Months[idx+1]
		frontVal, frontOK := s.metric(front)
		nextVal, nextOK := s.metric(next)
		if nextOK && (!frontOK || nextVal > frontVal) {
			s.current = next.ContractMonth
			return s.current, nil
		}
	}
	return s.current, nil
}
