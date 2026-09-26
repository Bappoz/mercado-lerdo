package stack

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// probeSQL is CPU-bound, single-threaded and independent of the dataset.
// EXPLAIN ANALYZE reports the server's own execution time, without psql's
// startup or the round trip.
const probeSQL = `SET max_parallel_workers_per_gather = 0;
EXPLAIN (ANALYZE, TIMING OFF) SELECT count(*) FROM generate_series(1, 1000000) g WHERE g % 7 = 3;
`

// probeRounds is how many times the probe runs; the median counts.
const probeRounds = 3

var executionTime = regexp.MustCompile(`Execution Time: ([0-9.]+) ms`)

// DBProbe times probeSQL in the sandbox's Postgres, under the player's
// database quota (evaluator.Prober).
func (s *Stack) DBProbe(ctx context.Context) (time.Duration, error) {
	var times []time.Duration
	for range probeRounds {
		res, err := s.d.Exec(ctx, s.Run, "postgres", 1,
			[]string{"psql", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "nexora", "-At"}, strings.NewReader(probeSQL))
		if err != nil {
			return 0, err
		}
		if res.ExitCode != 0 {
			return 0, fmt.Errorf("stack: db probe: psql exit %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
		}
		d, err := parseExecutionTime(res.Stdout)
		if err != nil {
			return 0, err
		}
		times = append(times, d)
	}
	slices.Sort(times)
	return times[len(times)/2], nil
}

func parseExecutionTime(explain string) (time.Duration, error) {
	m := executionTime.FindStringSubmatch(explain)
	if m == nil {
		return 0, fmt.Errorf("stack: db probe: no execution time in %q", explain)
	}
	ms, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, fmt.Errorf("stack: db probe: %w", err)
	}
	return time.Duration(math.Round(ms * float64(time.Millisecond))), nil
}
