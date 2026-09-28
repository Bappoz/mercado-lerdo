// Package evaluator turns measurements into per-dimension verdicts
// (docs/architecture/evaluator.md). It never uses metrics exposed by the app
// under evaluation.
package evaluator

import (
	"fmt"
	"slices"

	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

// Verdict of one objective or dimension.
type Verdict string

// Verdicts, ordered from best to worst.
const (
	Pass         Verdict = "PASS"
	Inconclusive Verdict = "INCONCLUSIVE"
	Fail         Verdict = "FAIL"
)

func (v Verdict) worse(o Verdict) Verdict {
	rank := map[Verdict]int{Pass: 0, Inconclusive: 1, Fail: 2}
	if rank[o] > rank[v] {
		return o
	}
	return v
}

// ObjectiveResult is one objective judged over all runs.
type ObjectiveResult struct {
	ID        string    `json:"id"`
	Dimension string    `json:"dimension"`
	Measure   string    `json:"measure"`
	Rule      string    `json:"rule"`
	Values    []float64 `json:"values"` // one per run (or a single value)
	Median    float64   `json:"median"`
	Verdict   Verdict   `json:"verdict"`
	Reason    string    `json:"reason,omitempty"`
}

// Judge applies the margin rule to the per-run values of one objective:
// PASS when every run clears the threshold by the margin, FAIL when every run
// misses it by the margin, INCONCLUSIVE otherwise (evaluator.md §4).
// Spread alone does not make a result inconclusive: a run far from the
// threshold is conclusive however noisy it is.
func Judge(o scenario.Objective, values []float64) ObjectiveResult {
	r := ObjectiveResult{ID: o.ID, Dimension: o.Dimension, Measure: o.Measure, Values: values, Median: median(values)}
	if len(values) == 0 {
		r.Verdict, r.Reason = Inconclusive, "sem medição"
		return r
	}
	var good, bad func(v float64) bool
	switch {
	case o.Below != nil:
		l := *o.Below
		r.Rule = fmt.Sprintf("%s < %g", o.Measure, l)
		good = func(v float64) bool { return v < l*(1-o.Margin) }
		bad = func(v float64) bool { return v > l*(1+o.Margin) || (o.Margin == 0 && v >= l) }
	case o.AtLeast != nil:
		l := *o.AtLeast
		r.Rule = fmt.Sprintf("%s >= %g", o.Measure, l)
		good = func(v float64) bool { return v >= l*(1+o.Margin) || (o.Margin == 0 && v >= l) }
		bad = func(v float64) bool { return v < l*(1-o.Margin) }
	default:
		r.Verdict, r.Reason = Inconclusive, "o objetivo não tem limite"
		return r
	}
	switch {
	case all(values, good):
		r.Verdict = Pass
	case all(values, bad):
		r.Verdict = Fail
	default:
		r.Verdict = Inconclusive
		r.Reason = fmt.Sprintf("as execuções %v ficaram dos dois lados do limite (%s, margem de %.0f%%)", values, r.Rule, o.Margin*100)
	}
	return r
}

// Dimensions folds objective results into the worst verdict per dimension.
func Dimensions(results []ObjectiveResult) map[string]Verdict {
	out := map[string]Verdict{}
	for _, r := range results {
		v, ok := out[r.Dimension]
		if !ok {
			v = Pass
		}
		out[r.Dimension] = v.worse(r.Verdict)
	}
	return out
}

func all(vs []float64, f func(float64) bool) bool {
	for _, v := range vs {
		if !f(v) {
			return false
		}
	}
	return true
}

func median(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	s := slices.Clone(vs)
	slices.Sort(s)
	if n := len(s); n%2 == 1 {
		return s[n/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}
