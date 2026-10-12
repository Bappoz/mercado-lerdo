package game

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/play"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

// ErrNoTutorial means the scenario has no guided mode, or it is over.
var ErrNoTutorial = errors.New("a Slotthy não está conduzindo um treinamento agora")

// counterGoals are the goals measured by a counter that must grow while the
// step is current: "run it now", not "you ran it once".
var counterGoals = []string{"query_stats", "explain", "quick_check"}

// tutorialFacts reads a counter goal's value (false when it cannot be read now).
type tutorialFacts func(goal string) (int64, bool)

// phaseStart is when a phase of s began (zero when it has not).
func phaseStart(s play.Session, phase string) time.Time {
	start, _ := phaseWindow(s, phase)
	return start
}

// tutorialDone is when the player did what step asks, if they did.
func tutorialDone(st scenario.TutorialStep, m play.TutorialMark, s play.Session, facts tutorialFacts, now time.Time) (time.Time, bool) {
	since := func(t time.Time) (time.Time, bool) {
		return t, !t.IsZero() && !t.Before(phaseStart(s, st.Phase))
	}
	switch st.Goal {
	case "view_pulse":
		return since(s.Viewed["pulse"])
	case "view_support":
		return since(s.Viewed["support"])
	case "open_shell":
		return since(s.Viewed["shell"])
	case "deploy":
		return since(s.DeployedAt)
	case "next":
		return s.IncidentAt, !s.IncidentAt.IsZero()
	case "test_pass":
		return s.DoneAt, !s.DoneAt.IsZero()
	case "continue":
		return m.OKAt, !m.OKAt.IsZero()
	case "answer":
		ok := len(m.Answers) > 0 && m.Answers[len(m.Answers)-1] == st.Answer
		return m.OKAt, ok
	case "add_migration":
		// Counted from the scenario's start, not the step's: writing it early is fine.
		if n, ok := facts(st.Goal); ok && n > 0 {
			return now, true
		}
	default: // counter goals
		if n, ok := facts(st.Goal); ok && n > m.Base {
			return now, true
		}
	}
	return time.Time{}, false
}

// evalTutorial advances the guided mode to now: it starts the current step,
// closes it when its goal is met (and starts the next, in the same call), and
// drops the calm steps the incident overtook. It returns the marks that
// changed (nil when nothing did).
func evalTutorial(steps []scenario.TutorialStep, s play.Session, facts tutorialFacts, now time.Time) map[string]play.TutorialMark {
	var changed map[string]play.TutorialMark
	set := func(id string, m play.TutorialMark) {
		if changed == nil {
			changed = map[string]play.TutorialMark{}
		}
		changed[id] = m
	}
	if s.Phase == "" {
		return nil
	}
	for _, st := range steps {
		m := s.Tutorial[st.ID]
		if !m.DoneAt.IsZero() || !m.SkippedAt.IsZero() {
			continue
		}
		incident := !s.IncidentAt.IsZero()
		if st.Phase == "calm" && incident && st.Goal != "next" {
			m.SkippedAt = now
			set(st.ID, m)
			continue
		}
		if st.Phase == "incident" && !incident {
			break
		}
		if m.StartedAt.IsZero() {
			m.StartedAt = now
			if slices.Contains(counterGoals, st.Goal) {
				m.Base, _ = facts(st.Goal)
			}
			set(st.ID, m)
		}
		if at, ok := tutorialDone(st, m, s, facts, now); ok {
			if at.IsZero() || at.Before(m.StartedAt) {
				at = m.StartedAt
			}
			m.DoneAt = at
			set(st.ID, m)
			continue
		}
		// A reset database restarts pg_stat_statements: count from there.
		if slices.Contains(counterGoals, st.Goal) {
			if n, ok := facts(st.Goal); ok && n < m.Base {
				m.Base = n
				set(st.ID, m)
			}
		}
		break
	}
	return changed
}

// TutorialView is what Slotthy says in the guided mode right now.
type TutorialView struct {
	Step     string       `json:"step"`  // "" once every step is done
	Index    int          `json:"index"` // 1-based
	Total    int          `json:"total"`
	Kind     string       `json:"kind"`
	Before   string       `json:"before,omitempty"` // what Slotthy says about the step just done
	Text     string       `json:"text"`
	Help     string       `json:"help,omitempty"` // once asked for
	CanHelp  bool         `json:"can_help"`
	Choices  []ChoiceView `json:"choices,omitempty"`
	Wrong    string       `json:"wrong,omitempty"` // after a wrong answer
	Continue bool         `json:"continue"`        // waits for "continuar"
	Done     bool         `json:"done"`            // every step is behind: the guided mode is over
	New      bool         `json:"new,omitempty"`   // the step started in this call: tell it once
	// Past is every step already behind, oldest first: a step can close before
	// the player read it (the goal was already met), so they can go back to it.
	Past []PastStep `json:"past,omitempty"`
}

// PastStep is a step that is behind, as Slotthy said it.
type PastStep struct {
	Step    string `json:"step"`
	Index   int    `json:"index"` // 1-based
	Kind    string `json:"kind"`
	Text    string `json:"text"`
	Done    string `json:"done,omitempty"`    // what Slotthy said when it was done
	Skipped bool   `json:"skipped,omitempty"` // a calm step the incident overtook
}

// ChoiceView is one answer Slotthy offers.
type ChoiceView struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// tutorialView renders the current step with render (words and live values).
func tutorialView(steps []scenario.TutorialStep, marks map[string]play.TutorialMark, render func(string) string) *TutorialView {
	v := &TutorialView{Total: len(steps)}
	for i, st := range steps {
		m := marks[st.ID]
		if skipped := !m.SkippedAt.IsZero(); skipped || !m.DoneAt.IsZero() {
			past := PastStep{Step: st.ID, Index: i + 1, Kind: st.Kind, Text: render(st.Text), Skipped: skipped}
			if !skipped {
				past.Done = render(st.Done)
				v.Before = past.Done
			}
			v.Past = append(v.Past, past)
			continue
		}
		if m.StartedAt.IsZero() {
			break // waiting for the incident
		}
		v.Step, v.Index, v.Kind, v.Text = st.ID, i+1, st.Kind, render(st.Text)
		v.CanHelp = st.Help != "" && m.HelpAt.IsZero()
		if !m.HelpAt.IsZero() {
			v.Help = render(st.Help)
		}
		for _, c := range st.Choices {
			v.Choices = append(v.Choices, ChoiceView{ID: c.ID, Text: render(c.Text)})
		}
		if n := len(m.Answers); n > 0 && m.Answers[n-1] != st.Answer {
			v.Wrong = render(st.Wrong)
		}
		v.Continue = st.Goal == "continue"
		return v
	}
	v.Done = allClosed(steps, marks)
	return v
}

func allClosed(steps []scenario.TutorialStep, marks map[string]play.TutorialMark) bool {
	for _, st := range steps {
		if m := marks[st.ID]; m.DoneAt.IsZero() && m.SkippedAt.IsZero() {
			return false
		}
	}
	return true
}

// tutorialFacts reads the counters the tutorial's goals need, lazily: the
// database is asked only while a step waits on it, and never when probe is
// false (a Production Test is measuring: ADR-0021).
func (g *Game) tutorialFacts(ctx context.Context, probe bool) tutorialFacts {
	var queried bool
	var stats, explains int64
	return func(goal string) (int64, bool) {
		switch goal {
		case "query_stats", "explain":
			st := g.running()
			if st == nil || !probe {
				return 0, false
			}
			if !queried {
				var err error
				if stats, explains, err = st.PlayerQueries(ctx); err != nil {
					g.log.Warn("tutorial: cannot read the player's queries", "err", err)
					return 0, false
				}
				queried = true
			}
			if goal == "explain" {
				return explains, true
			}
			return stats, true
		case "quick_check":
			return int64(g.Session().Checks), true
		case "add_migration":
			n, err := g.newMigrations(ctx)
			if err != nil {
				g.log.Warn("tutorial: cannot list the migrations", "err", err)
				return 0, false
			}
			return n, true
		}
		return 0, false
	}
}

// newMigrations counts the .sql files in the workspace's migrations/ that the
// scenario did not start with.
func (g *Game) newMigrations(ctx context.Context) (int64, error) {
	ws := g.Session().Workspace
	start, err := gitOutput(ctx, ws, "ls-tree", "--name-only", "sesim/scenario-start/"+filepath.Base(g.Pack().Dir), "migrations/")
	if err != nil {
		return 0, err
	}
	old := strings.Fields(start)
	now, err := filepath.Glob(filepath.Join(ws, "migrations", "*.sql"))
	if err != nil {
		return 0, err
	}
	var n int64
	for _, f := range now {
		if !slices.Contains(old, "migrations/"+filepath.Base(f)) {
			n++
		}
	}
	return n, nil
}

// Tutorial advances the guided mode to now and returns what Slotthy says; nil
// when the scenario has none. words names the tools for the view asking;
// probe false keeps it off the sandbox while a job measures it.
func (g *Game) Tutorial(ctx context.Context, words map[string]string, probe bool) (*TutorialView, error) {
	steps := g.Pack().Tutorial
	s := g.Session()
	if len(steps) == 0 || s.Phase == "" {
		return nil, nil
	}
	marks := maps.Clone(s.Tutorial)
	var fresh string
	closed := false // a step was done in this call
	if changed := evalTutorial(steps, s, g.tutorialFacts(ctx, probe), time.Now().UTC()); changed != nil {
		if err := g.update(func(s *play.Session) {
			if s.Tutorial == nil {
				s.Tutorial = map[string]play.TutorialMark{}
			}
			maps.Copy(s.Tutorial, changed)
		}); err != nil {
			return nil, err
		}
		if marks == nil {
			marks = map[string]play.TutorialMark{}
		}
		for id, m := range changed {
			old := marks[id]
			switch {
			case old.StartedAt.IsZero() && m.DoneAt.IsZero() && m.SkippedAt.IsZero():
				fresh = id
				g.Event("tutorial_step", map[string]any{"id": id, "outcome": "started"})
			case old.DoneAt.IsZero() && !m.DoneAt.IsZero():
				closed = true
				g.Event("tutorial_step", map[string]any{"id": id, "outcome": "done"})
			case old.SkippedAt.IsZero() && !m.SkippedAt.IsZero():
				g.Event("tutorial_step", map[string]any{"id": id, "outcome": "skipped"})
			}
			marks[id] = m
		}
	}
	render := func(t string) string { return g.render(ctx, t, words) }
	v := tutorialView(steps, marks, render)
	v.New = (fresh != "" && fresh == v.Step) || (closed && v.Done)
	return v, nil
}

// render fills a text's placeholders: the tools' names and the live values.
func (g *Game) render(ctx context.Context, text string, words map[string]string) string {
	vars := map[string]string{"workspace": g.Session().Workspace}
	maps.Copy(vars, words)
	if strings.Contains(text, "_url}}") {
		if st := g.running(); st != nil {
			vars["app_url"], _ = st.NginxURL(ctx)
			vars["prometheus_url"], _ = st.PrometheusURL(ctx)
		}
	}
	return strings.TrimSpace(play.RenderBriefing(text, vars))
}

// currentStep is the step the tutorial waits on.
func (g *Game) currentStep(ctx context.Context) (scenario.TutorialStep, error) {
	v, err := g.Tutorial(ctx, nil, true)
	if err != nil {
		return scenario.TutorialStep{}, err
	}
	if v == nil || v.Step == "" {
		return scenario.TutorialStep{}, ErrNoTutorial
	}
	i := slices.IndexFunc(g.Pack().Tutorial, func(st scenario.TutorialStep) bool { return st.ID == v.Step })
	return g.Pack().Tutorial[i], nil
}

// TutorialOK is the player's "continuar" on a step that waits for it.
func (g *Game) TutorialOK(ctx context.Context) error {
	st, err := g.currentStep(ctx)
	if err != nil {
		return err
	}
	if st.Goal != "continue" {
		return fmt.Errorf("a Slotthy está esperando outra coisa neste passo")
	}
	return g.markStep(st.ID, func(m *play.TutorialMark) { m.OKAt = time.Now().UTC() })
}

// TutorialHelp is the player asking Slotthy for help with a question.
func (g *Game) TutorialHelp(ctx context.Context) error {
	st, err := g.currentStep(ctx)
	if err != nil {
		return err
	}
	if st.Help == "" {
		return fmt.Errorf("este passo já diz tudo o que é preciso")
	}
	g.Event("tutorial_help", map[string]any{"id": st.ID})
	return g.markStep(st.ID, func(m *play.TutorialMark) {
		if m.HelpAt.IsZero() {
			m.HelpAt = time.Now().UTC()
		}
	})
}

// TutorialAnswer is the player's answer to a question; right is whether it was.
func (g *Game) TutorialAnswer(ctx context.Context, choice string) (right bool, err error) {
	st, err := g.currentStep(ctx)
	if err != nil {
		return false, err
	}
	if st.Goal != "answer" {
		return false, fmt.Errorf("a Slotthy não fez uma pergunta neste passo")
	}
	if !slices.ContainsFunc(st.Choices, func(c scenario.Choice) bool { return c.ID == choice }) {
		return false, fmt.Errorf("resposta desconhecida %q", choice)
	}
	right = choice == st.Answer
	g.Event("tutorial_answer", map[string]any{"id": st.ID, "right": right})
	return right, g.markStep(st.ID, func(m *play.TutorialMark) {
		m.Answers = append(m.Answers, choice)
		if right {
			m.OKAt = time.Now().UTC()
		}
	})
}

func (g *Game) markStep(id string, f func(*play.TutorialMark)) error {
	return g.update(func(s *play.Session) {
		if s.Tutorial == nil {
			s.Tutorial = map[string]play.TutorialMark{}
		}
		m := s.Tutorial[id]
		f(&m)
		s.Tutorial[id] = m
	})
}

// IsTutorial reports whether this scenario is played in the guided mode.
func (g *Game) IsTutorial() bool { return len(g.Pack().Tutorial) > 0 }
