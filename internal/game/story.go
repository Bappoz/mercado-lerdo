package game

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/play"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

// StoryMessage is one line of the story: an event, or the reply when its
// goal is met or its deadline passes.
type StoryMessage struct {
	ID    string    `json:"id"`
	Kind  string    `json:"kind"` // scenario.Kinds, or "reply"
	From  string    `json:"from"` // display name
	Title string    `json:"title"`
	Text  string    `json:"text"`
	At    time.Time `json:"at"`
	New   bool      `json:"new,omitempty"` // it happened in this call to Story: tell it once
}

// Deadline is the most urgent open request.
type Deadline struct {
	ID    string    `json:"id"`
	Kind  string    `json:"kind"`
	Title string    `json:"title"`
	From  string    `json:"from"`
	Goal  string    `json:"goal"`
	Until time.Time `json:"until"`
}

// StoryView is what the story has said so far, and what it waits for.
type StoryView struct {
	Messages []StoryMessage `json:"messages"`
	Deadline *Deadline      `json:"deadline,omitempty"`
}

// phaseWindow is when a phase of s was current: [start, end), with a zero
// start when it has not begun and a zero end while it lasts.
func phaseWindow(s play.Session, phase string) (start, end time.Time) {
	switch phase {
	case "calm":
		if s.IncidentAt.IsZero() {
			return s.StartedAt, s.DoneAt
		}
		return s.StartedAt, s.IncidentAt
	case "incident":
		return s.IncidentAt, s.DoneAt
	case "completed":
		return s.DoneAt, time.Time{}
	}
	return time.Time{}, time.Time{}
}

// firesAt is when e fires in s, if it has by now.
func firesAt(e scenario.StoryEvent, s play.Session, now time.Time) (time.Time, bool) {
	switch {
	case e.At != nil:
		start, end := phaseWindow(s, e.At.Phase)
		if start.IsZero() {
			return time.Time{}, false
		}
		// After counts the story's time, which stops while the clock is paused (ADR-0026).
		at, ok := clockAdd(s.Pauses, start, e.At.After)
		// Backfilled when nobody looked while it happened, but never after its phase ended.
		if !ok || at.After(now) || (!end.IsZero() && !at.Before(end)) {
			return time.Time{}, false
		}
		return at, true
	case e.On == "test_failed":
		return s.LastFailAt, !s.LastFailAt.IsZero()
	case e.On == "test_passed":
		return s.DoneAt, !s.DoneAt.IsZero()
	case e.On == "store_down":
		return s.CrashedAt, !s.CrashedAt.IsZero()
	}
	return time.Time{}, false
}

// goalDone is when the player did what e asks, after it was asked, if they
// did. storeUp is a live check, asked only when needed.
func goalDone(e scenario.StoryEvent, s play.Session, since, now time.Time, storeUp func() bool) (time.Time, bool) {
	after := func(t time.Time) (time.Time, bool) { return t, !t.IsZero() && !t.Before(since) }
	switch e.Goal {
	case "test_pass":
		return after(s.DoneAt)
	case "deploy":
		return after(s.DeployedAt)
	case "view_pulse":
		return after(s.Viewed["pulse"])
	case "view_support":
		return after(s.Viewed["support"])
	case "store_up":
		// "Back up" means after it went down: the crash comes a few seconds
		// after the incident starts, and the store is up until then.
		if !s.CrashedAt.IsZero() && storeUp() {
			return now, true
		}
	}
	return time.Time{}, false
}

// evalStory advances the story to now: it fires what is due and closes the
// goals that were met or whose deadline passed. It returns the marks that
// changed (nil when nothing did).
func evalStory(events []scenario.StoryEvent, s play.Session, now time.Time, storeUp func() bool) map[string]play.StoryMark {
	var changed map[string]play.StoryMark
	set := func(id string, m play.StoryMark) {
		if changed == nil {
			changed = map[string]play.StoryMark{}
		}
		changed[id] = m
	}
	for _, e := range events {
		m := s.Story[e.ID]
		if m.FiredAt.IsZero() {
			at, ok := firesAt(e, s, now)
			if !ok {
				continue
			}
			m.FiredAt = at
			set(e.ID, m)
		}
		if e.Goal == "" || !m.MetAt.IsZero() || !m.MissedAt.IsZero() {
			continue
		}
		// While the clock is paused the deadline does not come (ADR-0026).
		deadline, comes := clockAdd(s.Pauses, m.FiredAt, e.Within)
		if done, ok := goalDone(e, s, m.FiredAt, now, storeUp); ok && (e.Within == 0 || !comes || !done.After(deadline)) {
			m.MetAt = done
			set(e.ID, m)
		} else if e.Within > 0 && comes && now.After(deadline) {
			m.MissedAt = deadline
			set(e.ID, m)
		}
	}
	return changed
}

// baselinePlaceholder in a story text becomes the production numbers the
// player saw in the calm phase, or goes away when there are none.
const baselinePlaceholder = "{{baseline}}"

func withBaseline(text string, b *play.Baseline) string {
	if !strings.Contains(text, baselinePlaceholder) {
		return text
	}
	numbers := ""
	if b != nil {
		numbers = b.Markdown()
	}
	return strings.TrimSpace(strings.ReplaceAll(text, baselinePlaceholder, numbers))
}

// storyView renders the marks as messages, oldest first, and the most urgent
// open deadline. baseline fills the texts' {{baseline}}; pauses stretch the
// deadline (while the clock is paused, it is the one it had when it stopped).
func storyView(events []scenario.StoryEvent, marks map[string]play.StoryMark, baseline *play.Baseline, pauses []play.Pause) StoryView {
	v := StoryView{Messages: []StoryMessage{}}
	for _, e := range events {
		m, ok := marks[e.ID]
		if !ok || m.FiredAt.IsZero() {
			continue
		}
		from := scenario.Cast[e.From]
		v.Messages = append(v.Messages, StoryMessage{ID: "story-" + e.ID, Kind: e.Kind, From: from, Title: e.Title, Text: withBaseline(e.Text, baseline), At: m.FiredAt})
		switch {
		case !m.MetAt.IsZero():
			v.Messages = append(v.Messages, StoryMessage{ID: "story-" + e.ID + "-met", Kind: "reply", From: from, Title: "Re: " + e.Title, Text: withBaseline(e.Met, baseline), At: m.MetAt})
		case !m.MissedAt.IsZero():
			v.Messages = append(v.Messages, StoryMessage{ID: "story-" + e.ID + "-missed", Kind: "reply", From: from, Title: "Re: " + e.Title, Text: withBaseline(e.Missed, baseline), At: m.MissedAt})
		case e.Within > 0:
			until, _ := clockAdd(frozen(pauses), m.FiredAt, e.Within)
			d := &Deadline{ID: e.ID, Kind: e.Kind, Title: e.Title, From: from, Goal: e.Goal, Until: until}
			if v.Deadline == nil || d.Until.Before(v.Deadline.Until) {
				v.Deadline = d
			}
		}
	}
	slices.SortStableFunc(v.Messages, func(a, b StoryMessage) int { return a.At.Compare(b.At) })
	return v
}

// Story advances the scenario's story to now and returns it. Views call it
// when they read the game's state: the story needs no clock of its own.
func (g *Game) Story(ctx context.Context) (StoryView, error) {
	events := g.Pack().Events
	s := g.Session()
	if len(events) == 0 || s.Phase == "" {
		return StoryView{Messages: []StoryMessage{}}, nil
	}
	storeUp := func() bool {
		st := g.running()
		return st != nil && st.Healthy(ctx)
	}
	marks := maps.Clone(s.Story)
	fresh := map[string]bool{} // message IDs this call produced
	if changed := evalStory(events, s, time.Now().UTC(), storeUp); changed != nil {
		if err := g.update(func(s *play.Session) {
			if s.Story == nil {
				s.Story = map[string]play.StoryMark{}
			}
			maps.Copy(s.Story, changed)
		}); err != nil {
			return StoryView{}, err
		}
		if marks == nil {
			marks = map[string]play.StoryMark{}
		}
		for id, m := range changed {
			old := marks[id]
			outcome := "fired"
			if old.FiredAt.IsZero() {
				fresh["story-"+id] = true
			}
			switch {
			case old.MetAt.IsZero() && !m.MetAt.IsZero():
				outcome = "met"
				fresh["story-"+id+"-met"] = true
			case old.MissedAt.IsZero() && !m.MissedAt.IsZero():
				outcome = "missed"
				fresh["story-"+id+"-missed"] = true
			}
			marks[id] = m
			g.Event("story_event", map[string]any{"id": id, "outcome": outcome})
		}
	}
	v := storyView(events, marks, s.Baseline, s.Pauses)
	for i := range v.Messages {
		v.Messages[i].New = fresh[v.Messages[i].ID]
	}
	return v, nil
}
