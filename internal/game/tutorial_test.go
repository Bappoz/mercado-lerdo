package game

import (
	"strings"
	"testing"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/play"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

var tutorialSteps = []scenario.TutorialStep{
	{ID: "pulse", Phase: "calm", Kind: "guia", Text: "Abra {{status}}.", Goal: "view_pulse", Done: "Esse é o normal."},
	{ID: "code", Phase: "calm", Kind: "guia", Text: "Leia o código em {{workspace}}.", Goal: "continue"},
	{ID: "next", Phase: "calm", Kind: "guia", Text: "Quando quiser, {{next}}.", Goal: "next"},
	{ID: "who", Phase: "incident", Kind: "pergunta", Text: "Quem está no teto?", Goal: "answer", Help: "Olhe a CPU.",
		Choices: []scenario.Choice{{ID: "a", Text: "app"}, {ID: "b", Text: "postgres"}}, Answer: "b", Wrong: "Olhe de novo.", Done: "Isso: o banco."},
	{ID: "stats", Phase: "incident", Kind: "guia", Text: "Rode a consulta.", Goal: "query_stats"},
	{ID: "migration", Phase: "incident", Kind: "guia", Text: "Crie a migração.", Goal: "add_migration"},
	{ID: "test", Phase: "incident", Kind: "guia", Text: "Rode {{test}}.", Goal: "test_pass", Done: "Passou!"},
}

// facts is a fake reader of the counter goals.
type facts map[string]int64

func (f facts) read(goal string) (int64, bool) { n, ok := f[goal]; return n, ok }

// advance applies evalTutorial's changes to s, like Game.Tutorial does.
func advance(s *play.Session, f facts, now time.Time) map[string]play.TutorialMark {
	changed := evalTutorial(tutorialSteps, *s, f.read, now)
	if s.Tutorial == nil {
		s.Tutorial = map[string]play.TutorialMark{}
	}
	for id, m := range changed {
		s.Tutorial[id] = m
	}
	return changed
}

func current(s play.Session) string {
	return tutorialView(tutorialSteps, s.Tutorial, func(t string) string { return t }).Step
}

func TestTutorialWalksTheMission(t *testing.T) {
	t0 := time.Date(2026, 9, 27, 14, 0, 0, 0, time.UTC)
	at := func(m int) time.Time { return t0.Add(time.Duration(m) * time.Minute) }
	s := play.Session{Phase: play.Calm, StartedAt: t0}
	f := facts{"query_stats": 3, "add_migration": 0}

	advance(&s, f, at(0))
	if current(s) != "pulse" {
		t.Fatalf("first step = %q", current(s))
	}
	s.Viewed = map[string]time.Time{"pulse": at(1)}
	advance(&s, f, at(1))
	if current(s) != "code" || s.Tutorial["pulse"].DoneAt != at(1) {
		t.Fatalf("after the Pulse: step %q, marks %+v", current(s), s.Tutorial["pulse"])
	}
	if v := tutorialView(tutorialSteps, s.Tutorial, func(t string) string { return t }); v.Before != "Esse é o normal." || !v.Continue || v.Index != 2 {
		t.Errorf("view = %+v", v)
	}
	s.Tutorial["code"] = play.TutorialMark{StartedAt: s.Tutorial["code"].StartedAt, OKAt: at(2)}
	advance(&s, f, at(2))
	if current(s) != "next" {
		t.Fatalf("after continuar: %q", current(s))
	}
	// Incident steps wait for the incident.
	if advance(&s, f, at(3)) != nil {
		t.Error("nothing should change before the incident")
	}
	s.Phase, s.IncidentAt = play.Incident, at(4)
	advance(&s, f, at(4))
	if current(s) != "who" {
		t.Fatalf("after next: %q", current(s))
	}
	// A wrong answer keeps the question, and says so.
	m := s.Tutorial["who"]
	m.Answers = []string{"a"}
	s.Tutorial["who"] = m
	advance(&s, f, at(5))
	if v := tutorialView(tutorialSteps, s.Tutorial, func(t string) string { return t }); v.Step != "who" || v.Wrong != "Olhe de novo." || !v.CanHelp {
		t.Fatalf("after a wrong answer: %+v", v)
	}
	m.Answers, m.OKAt = append(m.Answers, "b"), at(6)
	s.Tutorial["who"] = m
	advance(&s, f, at(6))
	if current(s) != "stats" || s.Tutorial["stats"].Base != 3 {
		t.Fatalf("after the right answer: %q, base %d", current(s), s.Tutorial["stats"].Base)
	}
	// Queries run before the step do not count; a reset database rebases the counter.
	f["query_stats"] = 0
	advance(&s, f, at(7))
	if current(s) != "stats" || s.Tutorial["stats"].Base != 0 {
		t.Fatalf("after a reset: %q, base %d", current(s), s.Tutorial["stats"].Base)
	}
	f["query_stats"] = 1
	f["add_migration"] = 1 // written early: it counts as soon as the step starts
	advance(&s, f, at(8))
	if current(s) != "test" {
		t.Fatalf("after the query and the migration: %q", current(s))
	}
	s.Phase, s.DoneAt = play.Completed, at(20)
	advance(&s, f, at(20))
	if v := tutorialView(tutorialSteps, s.Tutorial, func(t string) string { return t }); !v.Done || v.Step != "" || v.Before != "Passou!" {
		t.Errorf("at the end: %+v", v)
	}
}

func TestTutorialDropsCalmStepsTheIncidentOvertook(t *testing.T) {
	t0 := time.Now().Add(-time.Hour)
	s := play.Session{Phase: play.Calm, StartedAt: t0}
	advance(&s, facts{}, t0)
	s.Phase, s.IncidentAt = play.Incident, t0.Add(time.Minute) // `next` straight away
	advance(&s, facts{}, t0.Add(2*time.Minute))
	if s.Tutorial["pulse"].SkippedAt.IsZero() || s.Tutorial["code"].SkippedAt.IsZero() {
		t.Errorf("calm steps not skipped: %+v", s.Tutorial)
	}
	if s.Tutorial["next"].DoneAt.IsZero() || current(s) != "who" {
		t.Errorf("next = %+v, current %q", s.Tutorial["next"], current(s))
	}
}

func TestTutorialViewOnlyAfterTheFirstStep(t *testing.T) {
	if changed := evalTutorial(tutorialSteps, play.Session{}, facts{}.read, time.Now()); changed != nil {
		t.Errorf("a workspace with no game has no tutorial: %+v", changed)
	}
}

func TestC1TutorialLoads(t *testing.T) {
	p, err := scenario.Load("../../scenarios/stage1/c1-slow-search")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tutorial) == 0 {
		t.Fatal("C1 is the tutorial mission")
	}
	questions := 0
	for _, st := range p.Tutorial {
		if st.Goal == "answer" {
			questions++
		}
		if strings.Contains(st.Text, "sesim ") {
			t.Errorf("%s names a CLI command: the views name the tools", st.ID)
		}
	}
	if questions < 2 {
		t.Errorf("C1 has %d questions; the player thinks at two points at least", questions)
	}
}
