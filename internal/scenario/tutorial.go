package scenario

import (
	"errors"
	"fmt"
	"slices"
)

// TutorialStep is one step of a scenario's guided mode (ADR-0024): Tabby says
// Text until the player does Goal, then Done. A question (kind pergunta) does
// not say how: Help does, when the player asks; its goal is usually an answer
// among Choices, and a wrong one gets Wrong back.
type TutorialStep struct {
	ID      string   `yaml:"id"`
	Phase   string   `yaml:"phase"` // calm | incident
	Kind    string   `yaml:"kind"`  // see TutorialKinds
	Text    string   `yaml:"text"`
	Goal    string   `yaml:"goal"` // see TutorialGoals
	Done    string   `yaml:"done"` // optional: what Tabby says once it is done
	Help    string   `yaml:"help"`
	Choices []Choice `yaml:"choices"` // goal answer
	Answer  string   `yaml:"answer"`  // the right choice's id
	Wrong   string   `yaml:"wrong"`   // what Tabby says to a wrong choice
}

// Choice is one answer to a question.
type Choice struct {
	ID   string `yaml:"id"`
	Text string `yaml:"text"`
}

// Kinds and goals of the tutorial's steps.
var (
	TutorialKinds = []string{"guia", "pergunta"}
	TutorialGoals = []string{
		"view_pulse", "view_support", "open_shell", // the player opened a view
		"query_stats", "explain", // the player's own queries in psql
		"add_migration", "quick_check", "deploy", "next", "test_pass", // the player's results
		"continue", "answer", // the player answered Tabby
	}
)

// ValidateTutorial checks the steps of one pack.
func ValidateTutorial(steps []TutorialStep) error {
	var errs []error
	seen := map[string]bool{}
	incident := false
	for i, s := range steps {
		fail := func(format string, a ...any) {
			errs = append(errs, fmt.Errorf("tutorial[%d] %s: %s", i, s.ID, fmt.Sprintf(format, a...)))
		}
		if s.ID == "" || seen[s.ID] {
			fail("needs a unique id")
		}
		seen[s.ID] = true
		switch s.Phase {
		case "calm":
			if incident {
				fail("a calm step cannot come after an incident step")
			}
		case "incident":
			incident = true
		default:
			fail("phase %q is not calm or incident", s.Phase)
		}
		if !slices.Contains(TutorialKinds, s.Kind) {
			fail("kind %q is not one of %v", s.Kind, TutorialKinds)
		}
		if s.Text == "" {
			fail("needs a text")
		}
		if !slices.Contains(TutorialGoals, s.Goal) {
			fail("goal %q is not one of %v", s.Goal, TutorialGoals)
		}
		if s.Goal == "next" && s.Phase != "calm" {
			fail("next ends the calm phase: it is a calm step")
		}
		if s.Kind == "pergunta" && s.Help == "" {
			fail("a question needs help for who is stuck")
		}
		if s.Goal == "answer" {
			ids := make([]string, 0, len(s.Choices))
			for _, c := range s.Choices {
				if c.ID == "" || c.Text == "" || slices.Contains(ids, c.ID) {
					fail("choices need unique ids and a text")
				}
				ids = append(ids, c.ID)
			}
			if len(ids) < 2 || !slices.Contains(ids, s.Answer) || s.Wrong == "" {
				fail("an answer needs two or more choices, the right one and what to say to a wrong one")
			}
		} else if len(s.Choices) > 0 || s.Answer != "" {
			fail("choices are for goal answer")
		}
	}
	return errors.Join(errs...)
}
