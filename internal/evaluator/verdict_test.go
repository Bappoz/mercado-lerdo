package evaluator

import (
	"testing"

	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

func ptr(f float64) *float64 { return &f }

func TestJudge(t *testing.T) {
	below := scenario.Objective{ID: "lat", Dimension: "latency", Measure: "search_p95_ms", Below: ptr(300), Margin: 0.1}
	atLeast := scenario.Objective{ID: "ok", Dimension: "correctness", Measure: "oracle_pass_ratio", AtLeast: ptr(1)}
	tests := []struct {
		name   string
		o      scenario.Objective
		values []float64
		want   Verdict
	}{
		{"all well below", below, []float64{15, 16, 40}, Pass},
		{"noisy but far below is still conclusive", below, []float64{10, 90, 200}, Pass},
		{"all well above", below, []float64{5000, 5000, 4400}, Fail},
		{"inside the margin band", below, []float64{280, 290, 285}, Inconclusive},
		{"straddling", below, []float64{250, 350, 260}, Inconclusive},
		{"no values", below, nil, Inconclusive},
		{"exact ratio without margin passes", atLeast, []float64{1}, Pass},
		{"ratio below without margin fails", atLeast, []float64{0.8}, Fail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Judge(tt.o, tt.values); got.Verdict != tt.want {
				t.Errorf("Judge(%v) = %s (%s), want %s", tt.values, got.Verdict, got.Reason, tt.want)
			}
		})
	}
}

func TestDimensionsTakesWorst(t *testing.T) {
	got := Dimensions([]ObjectiveResult{
		{Dimension: "latency", Verdict: Pass},
		{Dimension: "latency", Verdict: Inconclusive},
		{Dimension: "correctness", Verdict: Pass},
		{Dimension: "availability", Verdict: Fail},
		{Dimension: "availability", Verdict: Pass},
	})
	want := map[string]Verdict{"latency": Inconclusive, "correctness": Pass, "availability": Fail}
	for d, v := range want {
		if got[d] != v {
			t.Errorf("%s = %s, want %s", d, got[d], v)
		}
	}
}

func TestMedian(t *testing.T) {
	if m := median([]float64{3, 1, 2}); m != 2 {
		t.Errorf("median odd = %v", m)
	}
	if m := median([]float64{4, 1, 3, 2}); m != 2.5 {
		t.Errorf("median even = %v", m)
	}
}
