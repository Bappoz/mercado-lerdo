package scenario

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// StoryEvent is a message from someone at the company, fired by the story's
// clock or by the player's results: a request, a scolding, an emergency with
// a deadline, praise or news (docs/product/universe.md §6, ADR-0022). A goal
// is checked by the game; missing its deadline costs a bonus, never the mission.
type StoryEvent struct {
	ID    string `yaml:"id"`
	Kind  string `yaml:"kind"` // see Kinds
	From  string `yaml:"from"` // a key of Cast
	Title string `yaml:"title"`
	Text  string `yaml:"text"`
	// Exactly one trigger: a moment of a phase, or a result of the player.
	At *StoryAt `yaml:"at"`
	On string   `yaml:"on"` // see Triggers
	// Goal is what the player is asked to do; Within, by when (0: no hurry).
	Goal   string        `yaml:"goal"` // see Goals
	Within time.Duration `yaml:"within"`
	Met    string        `yaml:"met"`    // the reply when the goal is met in time
	Missed string        `yaml:"missed"` // the reply when the deadline passes
}

// StoryAt fires an event After the start of Phase, while that phase lasts.
type StoryAt struct {
	Phase string        `yaml:"phase"` // calm | incident | completed
	After time.Duration `yaml:"after"`
}

// Kinds, triggers and goals the game understands.
var (
	Kinds    = []string{"pedido", "bronca", "emergencia", "elogio", "noticia"}
	Triggers = []string{"test_failed", "test_passed", "store_down"}
	Goals    = []string{"test_pass", "store_up", "deploy", "view_pulse", "view_support"}
	// Cast is who writes to the player, by key (docs/product/universe.md §2).
	Cast = map[string]string{
		"alvaro":  "Álvaro (fundador)",
		"jessica": "Jéssica (suporte)",
		"otavio":  "Otávio (financeiro)",
	}
)

// ValidateStory checks the events of one pack.
func ValidateStory(events []StoryEvent) error {
	var errs []error
	seen := map[string]bool{}
	for i, e := range events {
		fail := func(format string, a ...any) {
			errs = append(errs, fmt.Errorf("events[%d] %s: %s", i, e.ID, fmt.Sprintf(format, a...)))
		}
		if e.ID == "" || seen[e.ID] {
			fail("needs a unique id")
		}
		seen[e.ID] = true
		if !slices.Contains(Kinds, e.Kind) {
			fail("kind %q is not one of %v", e.Kind, Kinds)
		}
		if _, ok := Cast[e.From]; !ok {
			fail("from %q is not in the cast", e.From)
		}
		if e.Title == "" || e.Text == "" {
			fail("needs a title and a text")
		}
		switch {
		case (e.At == nil) == (e.On == ""):
			fail("needs exactly one trigger: at or on")
		case e.At != nil && (!slices.Contains([]string{"calm", "incident", "completed"}, e.At.Phase) || e.At.After < 0):
			fail("at needs a phase (calm, incident, completed) and after >= 0")
		case e.On != "" && !slices.Contains(Triggers, e.On):
			fail("on %q is not one of %v", e.On, Triggers)
		}
		if e.Goal != "" && !slices.Contains(Goals, e.Goal) {
			fail("goal %q is not one of %v", e.Goal, Goals)
		}
		if e.Goal != "" && e.Met == "" {
			fail("a goal needs the reply when it is met")
		}
		if e.Within < 0 || (e.Within > 0 && (e.Goal == "" || e.Missed == "")) {
			fail("a deadline needs a goal and the reply when it is missed")
		}
		if e.Kind == "emergencia" && e.Within == 0 {
			fail("an emergency has a deadline")
		}
		if (e.Kind == "elogio" || e.Kind == "noticia") && e.Goal != "" {
			fail("praise and news ask for nothing")
		}
	}
	return errors.Join(errs...)
}
