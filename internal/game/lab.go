package game

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/evaluator"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/workload"
)

// LabProfiles are the load profiles the lab offers (workloads/nexora/*.js).
var LabProfiles = []string{"normal-day", "flash-sale", "rush", "crash-day"}

// LabParams is one experiment in the load lab.
type LabParams struct {
	Profile  string `json:"profile"`
	Rate     int    `json:"rate"`     // requests per second at peak
	Duration int    `json:"duration"` // seconds at peak, after a 10 s ramp
	Crash    bool   `json:"crash"`    // crash the app's first instance halfway
}

// Validate keeps experiments within what a laptop sandbox can take.
func (p LabParams) Validate() error {
	switch {
	case !slices.Contains(LabProfiles, p.Profile):
		return fmt.Errorf("perfil desconhecido %q", p.Profile)
	case p.Rate < 1 || p.Rate > 150:
		return errors.New("a taxa vai de 1 a 150 req/s")
	case p.Duration < 20 || p.Duration > 120:
		return errors.New("a duração vai de 20 a 120 s")
	}
	return nil
}

// LabEndpoint is what the clients saw of one page.
type LabEndpoint struct {
	Name      string  `json:"name"`
	Requests  int     `json:"requests"`
	ErrorRate float64 `json:"error_rate"`
	P50ms     float64 `json:"p50_ms"`
	P95ms     float64 `json:"p95_ms"`
	P99ms     float64 `json:"p99_ms"`
}

// LabResult is one experiment's outcome. It is not graded.
type LabResult struct {
	Params    LabParams            `json:"params"`
	Endpoints []LabEndpoint        `json:"endpoints"`
	ErrorRate float64              `json:"error_rate"`
	Crashed   bool                 `json:"crashed"`
	Payments  *evaluator.Integrity `json:"payments,omitempty"` // nil when no checkout ran
	Elapsed   time.Duration        `json:"elapsed"`
}

// labRamp is the lab's fixed ramp-up.
const labRamp = 10 * time.Second

// Lab runs a load experiment against production as it is now (no data
// reset, no grading). The story's traffic pauses meanwhile.
func (g *Game) Lab(ctx context.Context, p LabParams, say Say) (LabResult, error) {
	if err := p.Validate(); err != nil {
		return LabResult{}, err
	}
	st, err := g.requireStack()
	if err != nil {
		return LabResult{}, err
	}
	say(fmt.Sprintf("Laboratório: %s até %d req/s por %d s%s. O tráfego do dia pausa.", p.Profile, p.Rate, p.Duration,
		map[bool]string{true: ", derrubando a instância 1 no meio", false: ""}[p.Crash]))
	if err := st.StopBackground(ctx); err != nil {
		return LabResult{}, err
	}
	// The experiment starts from a store that is up, and leaves production
	// as it found it: a replica it crashed comes back.
	if err := st.StartApp(ctx); err != nil {
		return LabResult{}, err
	}
	defer func() {
		bg := context.WithoutCancel(ctx)
		if p.Crash {
			if err := st.StartApp(bg); err != nil {
				say("Não consegui religar a instância derrubada: " + err.Error())
			}
		}
		if err := g.startBackground(bg); err != nil {
			say("Não consegui retomar o tráfego: " + err.Error())
		}
	}()
	start := time.Now()
	id := fmt.Sprintf("lab-%d", start.Unix())
	test := scenario.Test{Profile: p.Profile, Rate: p.Rate, Ramp: labRamp, Duration: time.Duration(p.Duration) * time.Second}

	crashErr := make(chan error, 1)
	crashed := false
	if p.Crash {
		crashed = true
		loadCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		// Halfway through the peak, counted from the start of the load.
		at := labRamp + test.Duration/2
		go func() {
			select {
			case <-loadCtx.Done():
				crashErr <- nil
			case <-time.After(at):
				say("Derrubando a instância 1 do app…")
				crashErr <- st.Crash(loadCtx, "app", 1)
			}
		}()
	} else {
		crashErr <- nil
	}
	res, err := st.RunLoad(ctx, test, id)
	if err != nil {
		return LabResult{}, err
	}
	if cerr := <-crashErr; cerr != nil {
		say("A queda não pôde ser injetada: " + cerr.Error())
		crashed = false
	}
	out := LabResult{Params: p, Crashed: crashed, Elapsed: time.Since(start)}
	var reqs, failed int
	checkouts := 0
	for _, e := range res.Endpoints {
		out.Endpoints = append(out.Endpoints, labEndpoint(e))
		reqs += e.Requests
		failed += e.Failed
		if e.Name == "checkout" {
			checkouts = e.Requests
		}
	}
	if reqs > 0 {
		out.ErrorRate = float64(failed) / float64(reqs)
	}
	if checkouts > 0 {
		say("Conferindo cobranças e pedidos…")
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-time.After(12 * time.Second): // checkouts still running on the server
		}
		charges, err := st.Charges(ctx, id+"-")
		if err != nil {
			return out, err
		}
		orders, err := st.PaidOrders(ctx, id+"-")
		if err != nil {
			return out, err
		}
		in := evaluator.Reconcile(charges, orders)
		out.Payments = &in
	}
	g.Event("lab_run", map[string]any{"profile": p.Profile, "rate": p.Rate, "duration": p.Duration, "crash": p.Crash,
		"seconds": int(out.Elapsed.Seconds())})
	return out, nil
}

func labEndpoint(e workload.Endpoint) LabEndpoint {
	ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	return LabEndpoint{Name: e.Name, Requests: e.Requests, ErrorRate: e.FailRate(), P50ms: ms(e.P50), P95ms: ms(e.P95), P99ms: ms(e.P99)}
}
