package play

import (
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

// RenderBriefing fills {{name}} placeholders; unknown ones become "(indisponível)".
func RenderBriefing(md string, vars map[string]string) string {
	for k, v := range vars {
		if v == "" {
			v = "(indisponível)"
		}
		md = strings.ReplaceAll(md, "{{"+k+"}}", v)
	}
	return md
}

// CLIWords name the game's tools in the player's briefings, hints and
// debrief ({{status}}, {{next}}…) when they are read in the terminal.
var CLIWords = map[string]string{
	"status": "`sesim status`",
	"shell":  "`sesim shell`",
	"check":  "`sesim check`",
	"test":   "`sesim test`",
	"hint":   "`sesim hint`",
	"next":   "rode `sesim next`",
	// start_day is next as Tabby says it, away from the welcome mail.
	"start_day": "rode `sesim next`",
	"deploy":    "`sesim deploy`",
	"support":   "`sesim support`",
	"lab":       "`sesim lab`",
	// Where the player reads the topology and the catalog.
	"architecture": "`sesim topology`",
	"docs":         "`sesim docs`",
	// What Tabby recommends reading (ADR-0026).
	"reading": "`sesim reading`",
}

// NextHint returns the next hint after shown ones, or false when none are left.
func NextHint(hints []scenario.Hint, shown int) (scenario.Hint, bool) {
	if shown >= len(hints) {
		return scenario.Hint{}, false
	}
	return hints[shown], true
}

// Endpoint is what the player sees of one endpoint.
type Endpoint struct {
	Name      string  `json:"name"`
	RPS       float64 `json:"rps"`
	ErrorRate float64 `json:"error_rate"`
	P95s      float64 `json:"p95_s"` // p95 over the window, seconds
}

// Finite returns e with NaN and infinite values zeroed: a quantile over no
// requests is NaN, which JSON (the session file, the web UI) cannot carry.
func (e Endpoint) Finite() Endpoint {
	for _, v := range []*float64{&e.RPS, &e.ErrorRate, &e.P95s} {
		if math.IsNaN(*v) || math.IsInf(*v, 0) {
			*v = 0
		}
	}
	return e
}

// Signals is a snapshot of production, as the player sees it.
type Signals struct {
	WindowS   int                `json:"window_s"`
	Endpoints []Endpoint         `json:"endpoints"`
	CPU       map[string]float64 `json:"cpu"` // fraction of quota
}

// Endpoint returns the named endpoint (zero value when absent).
func (s Signals) Endpoint(name string) Endpoint {
	i := slices.IndexFunc(s.Endpoints, func(e Endpoint) bool { return e.Name == name })
	if i < 0 {
		return Endpoint{Name: name, P95s: math.NaN()}
	}
	return s.Endpoints[i]
}

// Write prints the snapshot. prev, when not nil, adds a before → now column.
func (s Signals) Write(w io.Writer, prev *Signals) {
	fmt.Fprintf(w, "últimos %ds, vistos pelos clientes\n\n", s.WindowS)
	fmt.Fprintf(w, "  %-10s %8s %8s %11s", "endpoint", "req/s", "erros", "p95")
	if prev != nil {
		fmt.Fprintf(w, "   %s", "p95 no check anterior")
	}
	fmt.Fprintln(w)
	for _, e := range s.Endpoints {
		fmt.Fprintf(w, "  %-10s %8.1f %7.1f%% %11s", e.Name, e.RPS, e.ErrorRate*100, fmtSeconds(e.P95s))
		if prev != nil {
			fmt.Fprintf(w, "   %s", fmtSeconds(prev.Endpoint(e.Name).P95s))
		}
		fmt.Fprintln(w)
	}
	names := make([]string, 0, len(s.CPU))
	for n := range s.CPU {
		names = append(names, n)
	}
	slices.Sort(names)
	fmt.Fprintf(w, "\nCPU em relação ao limite de cada container\n\n")
	for _, n := range names {
		fmt.Fprintf(w, "  %-18s %4.0f%%  %s\n", n, s.CPU[n]*100, bar(s.CPU[n]))
	}
}

// Markdown lists the snapshot in the units of `sesim status` and the Pulse,
// so the player can compare it with what they see later.
func (b Baseline) Markdown() string {
	var sb strings.Builder
	sb.WriteString("O nosso normal, do jeito que você viu:\n\n")
	for _, e := range b.Endpoints {
		fmt.Fprintf(&sb, "- **%s**: p95 %s · %.1f req/s · %.1f%% de erros\n", e.Name, fmtSeconds(e.P95s), e.RPS, e.ErrorRate*100)
	}
	names := slices.Sorted(maps.Keys(b.CPU))
	cpu := make([]string, 0, len(names))
	for _, n := range names {
		cpu = append(cpu, fmt.Sprintf("%s %.0f%%", n, b.CPU[n]*100))
	}
	if len(cpu) > 0 {
		fmt.Fprintf(&sb, "- **CPU** (em relação ao limite de cada container): %s\n", strings.Join(cpu, " · "))
	}
	return strings.TrimSpace(sb.String())
}

func fmtSeconds(s float64) string {
	switch {
	case math.IsNaN(s):
		return "—"
	case s >= 1:
		return fmt.Sprintf("%.2f s", s)
	default:
		return fmt.Sprintf("%.0f ms", s*1000)
	}
}

func bar(f float64) string {
	n := int(math.Round(math.Min(f, 1) * 20))
	return "[" + strings.Repeat("#", n) + strings.Repeat(".", 20-n) + "]"
}
