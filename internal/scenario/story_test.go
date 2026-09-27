package scenario

import (
	"strings"
	"testing"
	"time"
)

func TestValidateStory(t *testing.T) {
	ok := StoryEvent{ID: "live", Kind: "emergencia", From: "alvaro", Title: "Live", Text: "Rápido!",
		At: &StoryAt{Phase: "incident", After: time.Minute}, Goal: "test_pass", Within: 30 * time.Minute,
		Met: "Salvou!", Missed: "Perdemos a live."}
	if err := ValidateStory([]StoryEvent{ok}); err != nil {
		t.Fatalf("a well-formed emergency: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*StoryEvent)
		want   string
	}{
		{"unknown kind", func(e *StoryEvent) { e.Kind = "fofoca" }, "kind"},
		{"stranger", func(e *StoryEvent) { e.From = "kleber" }, "cast"},
		{"two triggers", func(e *StoryEvent) { e.On = "test_failed" }, "exactly one trigger"},
		{"no trigger", func(e *StoryEvent) { e.At = nil }, "exactly one trigger"},
		{"unknown phase", func(e *StoryEvent) { e.At.Phase = "lunch" }, "phase"},
		{"unknown goal", func(e *StoryEvent) { e.Goal = "fix_everything" }, "goal"},
		{"deadline without a missed reply", func(e *StoryEvent) { e.Missed = "" }, "deadline"},
		{"emergency without a deadline", func(e *StoryEvent) { e.Within = 0 }, "emergency"},
		{"praise that asks", func(e *StoryEvent) { e.Kind = "elogio" }, "ask for nothing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := ok
			at := *ok.At
			e.At = &at
			tc.mutate(&e)
			if err := ValidateStory([]StoryEvent{e}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
	if err := ValidateStory([]StoryEvent{ok, ok}); err == nil || !strings.Contains(err.Error(), "unique") {
		t.Errorf("duplicate ids: %v", err)
	}
}

// Every Stage 1 pack tells its story: the events load and validate.
func TestStageOnePacksHaveStories(t *testing.T) {
	for _, dir := range []string{"c1-slow-search", "c2-connection-pressure", "c3-instance-failure"} {
		p, err := Load("../../scenarios/stage1/" + dir)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		kinds := map[string]bool{}
		for _, e := range p.Events {
			kinds[e.Kind] = true
		}
		if !kinds["emergencia"] || !kinds["bronca"] || !kinds["elogio"] {
			t.Errorf("%s: story kinds %v, want at least an emergency, a scolding and praise", dir, kinds)
		}
	}
}
