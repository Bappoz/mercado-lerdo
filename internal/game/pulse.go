package game

import (
	"context"
	"fmt"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/telemetry"
)

// Series is one line of a chart: [unix seconds, value] points.
type Series struct {
	Name   string       `json:"name"`
	Points [][2]float64 `json:"points"`
}

// Pulse is the recent history of production as the player sees it: the
// story's traffic seen by the clients, and the CPU of the player's containers.
type Pulse struct {
	From   int64    `json:"from"`
	To     int64    `json:"to"`
	StepS  int      `json:"step_s"`
	P95    []Series `json:"p95"`    // per endpoint, seconds
	RPS    []Series `json:"rps"`    // per endpoint
	Errors []Series `json:"errors"` // per endpoint, fraction of requests
	CPU    []Series `json:"cpu"`    // per container, fraction of its quota
}

// pulseRate is the rate window of each point: long enough to smooth one
// scrape interval, short enough to show a change within a minute.
const pulseRate = "30s"

// Pulse reads the last span of production at step resolution. The story's
// traffic of every phase is included (a Production Test's is not); the CPU
// needs WatchStats running.
func (g *Game) Pulse(ctx context.Context, span, step time.Duration) (Pulse, error) {
	st, err := g.requireStack()
	if err != nil {
		return Pulse{}, err
	}
	end := time.Now()
	start := end.Add(-span)
	out := Pulse{From: start.Unix(), To: end.Unix(), StepS: int(step.Seconds())}
	sel := fmt.Sprintf(`testid=~"%s.*",name!="setup_login",name!=""`, backgroundPrefix)
	q := signalQueries(sel, pulseRate)
	containers := `container=~"nginx|app|postgres|pgbouncer"`
	// Replicas together: busy cores over the cores all replicas may use.
	q["cpu"] = fmt.Sprintf(`sum by (container) (rate(sesim_container_cpu_usage_seconds_total{%s}[%s])) / on(container) sum by (container) (sesim_container_cpu_quota_cores{%s})`,
		containers, pulseRate, containers)
	for kind, dst := range map[string]*[]Series{"p95": &out.P95, "rps": &out.RPS, "err": &out.Errors, "cpu": &out.CPU} {
		label := "name"
		if kind == "cpu" {
			label = "container"
		}
		rs, err := st.QueryRange(ctx, q[kind], start, end, step)
		if err != nil {
			return Pulse{}, err
		}
		*dst = toSeries(rs, label)
	}
	return out, nil
}

func toSeries(rs []telemetry.RangeSeries, label string) []Series {
	out := make([]Series, 0, len(rs))
	for _, r := range rs {
		out = append(out, Series{Name: r.Labels[label], Points: r.Points})
	}
	return out
}
