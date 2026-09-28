package evaluator

import (
	"context"
	"fmt"
	"slices"
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
	Excused     bool    `json:"excused"` // a verdict became INCONCLUSIVE because of the machine
}

// Calibration (ADR-0021). The reference is the median probe of the machine
// the scenarios were calibrated on, plugged in. The floor stays below the
// smallest slowdown at which a reference solution that passes there could
// fail (SlowdownTolerance: 1.34, C3 redundancy-only's error_rate), less the
// probe's own noise: 267 × 1.34 / 1.10 ≈ 325 ms.
const (
	DefaultProbeReference = 267 * time.Millisecond
	DefaultProbeLimit     = 325 * time.Millisecond
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
		r.Reason = fmt.Sprintf("reprova mesmo com o limite esticado %.1fx para esta máquina mais lenta", h.Slowdown)
		return r
	}
	r.Verdict = Inconclusive
	h.Excused = true
	r.Reason = fmt.Sprintf("a máquina rodou %.1fx mais devagar do que a calibração dos cenários (teste do banco: %.0f ms; limite: %.0f ms)",
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

// Tolerance is how much slower than the machine that measured a report a
// host may be before a speed measure that passed there could fail.
type Tolerance struct {
	Slowdown  float64 // the smallest such slowdown
	Objective string  // the objective closest to failing
}

// probeNoise is the spread of the median probe between Production Tests on
// the same plugged-in machine (235–294 ms around 267 ms: the CPU's boost).
const probeNoise = 0.10

// SlowdownTolerance is the smallest slowdown k at which k times the worst run
// of a speed objective that passed in rep would miss its threshold by the
// margin, assuming the measure grows with the slowdown (ADR-0021). ok is
// false when no speed objective passed.
func SlowdownTolerance(objectives []scenario.Objective, rep Report) (t Tolerance, ok bool) {
	for _, r := range rep.Objectives {
		i := slices.IndexFunc(objectives, func(o scenario.Objective) bool { return o.ID == r.ID })
		if i < 0 || r.Verdict != Pass || len(r.Values) == 0 || !speedSensitive(r.Measure) || objectives[i].Below == nil {
			continue
		}
		worst := slices.Max(r.Values)
		if worst <= 0 {
			continue
		}
		o := objectives[i]
		if k := *o.Below * (1 + o.Margin) / worst; !ok || k < t.Slowdown {
			t, ok = Tolerance{Slowdown: k, Objective: o.ID}, true
		}
	}
	return t, ok
}

// FloorFor is the probe limit that tolerance allows on a machine whose
// median probe is referenceMS: below the slowdown, less the probe's noise.
func FloorFor(t Tolerance, referenceMS float64) float64 {
	return referenceMS * t.Slowdown / (1 + probeNoise)
}
