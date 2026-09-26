package stack

import (
	"testing"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/evaluator"
)

var _ evaluator.Prober = (*Stack)(nil)

func TestParseExecutionTime(t *testing.T) {
	out := "SET\nAggregate  (cost=17500.00..17500.01 rows=1 width=8) (actual rows=1.00 loops=1)\n" +
		"  ->  Function Scan on generate_series g  (actual rows=142857.00 loops=1)\nPlanning Time: 0.061 ms\nExecution Time: 694.148 ms\n"
	d, err := parseExecutionTime(out)
	if err != nil || d != 694148*time.Microsecond {
		t.Errorf("parse = %v, %v", d, err)
	}
	if _, err := parseExecutionTime("ERROR: canceled"); err == nil {
		t.Error("output without a time must be an error")
	}
}
