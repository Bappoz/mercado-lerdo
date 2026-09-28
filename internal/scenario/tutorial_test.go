package scenario

import (
	"strings"
	"testing"
)

func TestValidateTutorial(t *testing.T) {
	ok := TutorialStep{ID: "a", Phase: "calm", Kind: "guia", Text: "t", Goal: "view_pulse"}
	q := TutorialStep{ID: "q", Phase: "incident", Kind: "pergunta", Text: "t", Goal: "answer", Help: "h",
		Choices: []Choice{{ID: "a", Text: "x"}, {ID: "b", Text: "y"}}, Answer: "b", Wrong: "w"}
	if err := ValidateTutorial([]TutorialStep{ok, q}); err != nil {
		t.Fatalf("valid tutorial rejected: %v", err)
	}
	with := func(f func(*TutorialStep)) TutorialStep { s := q; f(&s); return s }
	for _, tt := range []struct {
		name  string
		steps []TutorialStep
		want  string
	}{
		{"duplicate id", []TutorialStep{ok, ok}, "unique id"},
		{"calm after incident", []TutorialStep{q, ok}, "calm step cannot come after"},
		{"unknown goal", []TutorialStep{with(func(s *TutorialStep) { s.Goal = "win" })}, "goal"},
		{"question without help", []TutorialStep{with(func(s *TutorialStep) { s.Help = "" })}, "needs help"},
		{"answer not a choice", []TutorialStep{with(func(s *TutorialStep) { s.Answer = "z" })}, "two or more choices"},
		{"one choice", []TutorialStep{with(func(s *TutorialStep) { s.Choices = s.Choices[1:] })}, "two or more choices"},
		{"choices without answer goal", []TutorialStep{with(func(s *TutorialStep) { s.Goal = "continue" })}, "choices are for goal answer"},
		{"next in the incident", []TutorialStep{with(func(s *TutorialStep) { s.Goal, s.Choices, s.Answer = "next", nil, "" })}, "calm step"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateTutorial(tt.steps); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}
