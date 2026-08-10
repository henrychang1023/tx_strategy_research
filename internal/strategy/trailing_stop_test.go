package strategy

import (
	"testing"

	"strategy/internal/roll"
)

func f(v float64) *float64 { return &v }

func adjBars(values ...float64) []roll.ContinuousBar {
	bars := make([]roll.ContinuousBar, len(values))
	for i, v := range values {
		bars[i] = roll.ContinuousBar{AdjustedClose: f(v)}
	}
	return bars
}

func TestTrailingStopExitIndexTriggers(t *testing.T) {
	// peak reaches 120 at i2; 10% trailing stop threshold is 108; i4=107
	// breaches it. The later rise at i5 must not move the exit index back.
	bars := adjBars(100, 110, 120, 115, 107, 130)

	got := TrailingStopExitIndex(bars, 0.10)
	if got != 4 {
		t.Errorf("TrailingStopExitIndex = %d, want 4", got)
	}
}

func TestTrailingStopExitIndexNeverTriggers(t *testing.T) {
	bars := adjBars(100, 105, 110, 120)

	got := TrailingStopExitIndex(bars, 0.10)
	want := len(bars) - 1
	if got != want {
		t.Errorf("TrailingStopExitIndex = %d, want %d (monotonic rise, stop never triggers)", got, want)
	}
}

func TestTrailingStopExitIndexUsesAdjustedCloseNotRawClose(t *testing.T) {
	// Raw Close shows a large roll-day-style drop that would falsely trigger
	// a 10% stop if the function mistakenly used it; AdjustedClose (the
	// gap-free series) keeps rising throughout, so the stop must not fire.
	bars := []roll.ContinuousBar{
		{Close: f(100), AdjustedClose: f(100)},
		{Close: f(60), AdjustedClose: f(105)}, // raw gap, but adjusted still rises
		{Close: f(65), AdjustedClose: f(110)},
	}

	got := TrailingStopExitIndex(bars, 0.10)
	want := len(bars) - 1
	if got != want {
		t.Errorf("TrailingStopExitIndex = %d, want %d (must ignore the raw-Close roll gap)", got, want)
	}
}

func TestTrailingStopExitIndexEmpty(t *testing.T) {
	if got := TrailingStopExitIndex(nil, 0.10); got != -1 {
		t.Errorf("TrailingStopExitIndex(nil) = %d, want -1", got)
	}
}
