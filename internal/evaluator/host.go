package evaluator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

// Prober times a fixed CPU-bound query in the sandbox's database, under the
// same quota as the player's (stack.Stack implements it). Verdicts on speed
// depend on how fast the host computes: a laptop in power-saver mode fails
// solutions that pass on the same laptop plugged in (ADR-0021).
type Prober interface {
	DBProbe(ctx context.Context) (time.Duration, error)
}

// HostCheck says whether the machine was fast enough to judge on speed.
type HostCheck struct {
	ProbeMS     float64 `json:"db_probe_ms"`  // median over the runs: one slow probe is noise
	LimitMS     float64 `json:"limit_ms"`     // the floor
	ReferenceMS float64 `json:"reference_ms"` // the calibration machine's probe
	Slowdown    float64 `json:"slowdown"`     // ProbeMS / ReferenceMS
	Slow        bool    `json:"slow"`
}

// Calibration (ADR-0021). The reference is the median probe of the machine
// the scenarios were calibrated on, plugged in. The floor is where the
// reference solution closest to a speed threshold (C1 trigram, db_cpu_p90
// at 0.52 against a PASS bound of 0.72) stops clearing it, if its CPU use
// grows with the slowdown.
const (
	DefaultProbeReference = 305 * time.Millisecond
	DefaultProbeLimit     = 400 * time.Millisecond
)

// speedSensitive reports whether a measure can only get worse on a slower
// host: latency, CPU use against a quota, and the errors of a saturated system.
func speedSensitive(measure string) bool {
	return strings.HasSuffix(measure, "_ms") || strings.HasPrefix(measure, "db_cpu") || measure == "error_rate"
}

// excuseSlowHost re-judges a miss on a speed measure when the host was
// below the floor, with the threshold stretched by the host's slowdown. A
// miss the slowdown explains becomes INCONCLUSIVE; one beyond it stays FAIL.
// A PASS stands: the solution cleared the bar with a handicap. Measures that
// do not depend on speed keep their verdict.
func excuseSlowHost(o scenario.Objective, r ObjectiveResult, h *HostCheck) ObjectiveResult {
	if h == nil || !h.Slow || r.Verdict == Pass || !speedSensitive(o.Measure) || o.Below == nil {
		return r
	}
	stretched := o
	stretched.Below = new(*o.Below * h.Slowdown)
	if Judge(stretched, r.Values).Verdict == Fail {
		r.Verdict = Fail
		r.Reason = fmt.Sprintf("fails even with the threshold stretched %.1fx for this slower machine", h.Slowdown)
		return r
	}
	r.Verdict = Inconclusive
	r.Reason = fmt.Sprintf("the machine ran %.1fx slower than the scenarios were calibrated for (database probe %.0f ms, limit %.0f ms)",
		h.Slowdown, h.ProbeMS, h.LimitMS)
	return r
}

func hostCheck(runs []Run, reference, limit time.Duration) *HostCheck {
	var probes []float64
	for _, r := range runs {
		if r.ProbeMS > 0 {
			probes = append(probes, r.ProbeMS)
		}
	}
	if len(probes) == 0 {
		return nil // the env cannot probe, or every probe failed
	}
	h := &HostCheck{ProbeMS: median(probes), LimitMS: float64(limit.Milliseconds()), ReferenceMS: float64(reference.Milliseconds())}
	h.Slowdown = h.ProbeMS / h.ReferenceMS
	h.Slow = h.ProbeMS > h.LimitMS
	return h
}
