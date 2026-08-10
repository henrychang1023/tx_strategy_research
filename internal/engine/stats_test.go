package engine

import "testing"

func TestMaxDrawdown(t *testing.T) {
	cases := []struct {
		name string
		in   []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5}, 0},
		{"monotonic up", []float64{1, 2, 3}, 0},
		{"peak then trough then new peak", []float64{100, 90, 120, 80, 130}, 40},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := maxDrawdown(c.in); got != c.want {
				t.Errorf("maxDrawdown(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestSharpeDegenerateCases(t *testing.T) {
	if got := sharpe(nil); got != 0 {
		t.Errorf("sharpe(nil) = %v, want 0", got)
	}
	if got := sharpe([]float64{1}); got != 0 {
		t.Errorf("sharpe(single value) = %v, want 0", got)
	}
	if got := sharpe([]float64{1, 1, 1, 1}); got != 0 {
		t.Errorf("sharpe(constant series) = %v, want 0 (zero variance guard)", got)
	}
}

func TestSharpeSign(t *testing.T) {
	if got := sharpe([]float64{1, 2, 1, 2, 1, 2}); got <= 0 {
		t.Errorf("sharpe of a consistently-positive series = %v, want > 0", got)
	}
	if got := sharpe([]float64{-1, -2, -1, -2, -1, -2}); got >= 0 {
		t.Errorf("sharpe of a consistently-negative series = %v, want < 0", got)
	}
}
