package cli

import (
	"strings"
	"testing"

	"github.com/Bappoz/software-engineering-simulator/internal/evaluator"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

func TestWriteMatrix(t *testing.T) {
	cases := []*verifyCase{
		{Name: "broken", Expect: scenario.Expect{"latency": "FAIL", "correctness": "PASS"},
			Rounds: []map[string]evaluator.Verdict{
				{"latency": "FAIL", "correctness": "PASS"},
				{"latency": "FAIL", "correctness": "PASS"},
			}},
		{Name: "trigram", Expect: scenario.Expect{"latency": "PASS"},
			Rounds: []map[string]evaluator.Verdict{{"latency": "PASS"}, {"latency": "INCONCLUSIVE"}}},
	}
	var b strings.Builder
	mismatches, flips, cells := writeMatrix(&b, cases)
	if mismatches != 1 || flips != 1 || cells != 3 {
		t.Errorf("mismatches=%d flips=%d cells=%d, want 1/1/3\n%s", mismatches, flips, cells, b.String())
	}
	if !strings.Contains(b.String(), "PASS INCONCLUSIVE") || !strings.Contains(b.String(), "UNEXPECTED") {
		t.Errorf("matrix output:\n%s", b.String())
	}
}
