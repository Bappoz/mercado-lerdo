package game

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/play"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

func TestClockAdd(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	at := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Minute) }
	closed := []play.Pause{{From: at(10), To: at(15)}, {From: at(30), To: at(40)}}
	open := []play.Pause{{From: at(10), To: at(15)}, {From: at(30)}}

	for _, tc := range []struct {
		name   string
		pauses []play.Pause
		from   time.Time
		d      time.Duration
		want   time.Time
		never  bool // the moment does not come while the clock is paused
	}{
		{name: "no pauses", from: at(0), d: 5 * time.Minute, want: at(5)},
		{name: "before the first pause", pauses: closed, from: at(0), d: 5 * time.Minute, want: at(5)},
		{name: "ends as the pause begins", pauses: closed, from: at(0), d: 10 * time.Minute, want: at(10)},
		{name: "a pause in the way pushes it", pauses: closed, from: at(0), d: 12 * time.Minute, want: at(17)},
		{name: "two pauses in the way", pauses: closed, from: at(0), d: 26 * time.Minute, want: at(41)},
		{name: "starting inside a pause counts from its end", pauses: closed, from: at(12), d: 2 * time.Minute, want: at(17)},
		{name: "a pause behind does not count", pauses: closed, from: at(20), d: 5 * time.Minute, want: at(25)},
		{name: "an open pause in the way", pauses: open, from: at(20), d: 15 * time.Minute, never: true},
		{name: "before the open pause", pauses: open, from: at(20), d: 5 * time.Minute, want: at(25)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := clockAdd(tc.pauses, tc.from, tc.d)
			if ok == tc.never || (ok && !got.Equal(tc.want)) {
				t.Errorf("clockAdd = %v, %v; want %v, never=%v", got, ok, tc.want, tc.never)
			}
		})
	}
}

// While the player reads with the clock paused, the story waits: nothing
// fires and no deadline passes; the time comes back when they resume.
func TestStoryWaitsWhileTheClockIsPaused(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	at := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Minute) }
	live := scenario.StoryEvent{ID: "live", Kind: "emergencia", From: "alvaro", Title: "t", Text: "x",
		At: &scenario.StoryAt{Phase: "incident", After: 2 * time.Minute}, Goal: "test_pass", Within: 20 * time.Minute, Met: "salvou", Missed: "perdemos"}
	evs := []scenario.StoryEvent{live}
	never := func() bool { return false }
	s := play.Session{Phase: play.Incident, StartedAt: t0, IncidentAt: at(10)}

	// Paused one minute into the incident: the emergency due at +2 min does not arrive.
	s.Pauses = []play.Pause{{From: at(11)}}
	if changed := evalStory(evs, s, at(30), never); changed != nil {
		t.Fatalf("fired while paused: %+v", changed)
	}
	// Resumed 19 minutes later: it arrives one unpaused minute after that.
	s.Pauses[0].To = at(30)
	if changed := evalStory(evs, s, at(30), never); changed != nil {
		t.Fatalf("fired the moment the clock resumed: %+v", changed)
	}
	changed := evalStory(evs, s, at(32), never)
	if got := changed["live"]; !got.FiredAt.Equal(at(31)) || !got.MissedAt.IsZero() {
		t.Fatalf("after resuming: %+v", got)
	}
	s.Story = changed

	// A pause with the deadline open stretches it: 20 min of play, not of wall time.
	s.Pauses = append(s.Pauses, play.Pause{From: at(41)})
	if changed := evalStory(evs, s, at(120), never); changed != nil {
		t.Fatalf("the deadline passed while paused: %+v", changed)
	}
	if d := storyView(evs, s.Story, nil, s.Pauses).Deadline; d == nil || !d.Until.Equal(at(51)) {
		t.Errorf("deadline while paused = %+v, want the one it had when the clock stopped", d)
	}
	s.Pauses[1].To = at(100)
	if d := storyView(evs, s.Story, nil, s.Pauses).Deadline; d == nil || !d.Until.Equal(at(110)) {
		t.Errorf("deadline after resuming = %+v, want 10:%d", d, 110)
	}
	// A PASS inside the stretched deadline meets it; without the pause it would be late.
	done := s
	done.Phase, done.DoneAt = play.Completed, at(105)
	if got := evalStory(evs, done, at(120), never)["live"]; !got.MetAt.Equal(at(105)) {
		t.Errorf("a PASS within the stretched deadline: %+v", got)
	}
	if got := evalStory(evs, s, at(120), never)["live"]; !got.MissedAt.Equal(at(110)) {
		t.Errorf("the stretched deadline still passes: %+v", got)
	}
}

func TestPauseAndResume(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	s := play.Session{Phase: play.Calm, StartedAt: t0}
	if !pauseClock(&s, t0.Add(time.Minute)) || pauseClock(&s, t0.Add(2*time.Minute)) {
		t.Fatalf("pausing twice: %+v", s.Pauses)
	}
	if !clockPaused(s) {
		t.Fatal("not paused after pausing")
	}
	if d, ok := resumeClock(&s, t0.Add(4*time.Minute)); !ok || d != 3*time.Minute {
		t.Errorf("resumed after %v, %v", d, ok)
	}
	if _, ok := resumeClock(&s, t0.Add(5*time.Minute)); ok || clockPaused(s) || len(s.Pauses) != 1 {
		t.Errorf("resuming twice: %+v", s.Pauses)
	}
}

// The clock is part of the save: another process (the CLI next to the UI)
// sees it stopped, and a finished mission has no clock to stop.
func TestGameClockAndReading(t *testing.T) {
	g := newTestGame(t)
	ctx := context.Background()

	list := g.Reading(ctx, play.CLIWords)
	if len(list) == 0 {
		t.Fatal("C1 recommends reading")
	}
	for _, r := range list {
		if strings.Contains(r.Why, "{{") {
			t.Errorf("%s: unfilled placeholder in %q", r.ID, r.Why)
		}
	}
	if err := g.OpenedReading("nope"); err == nil {
		t.Error("an unknown reading was accepted")
	}
	if err := g.OpenedReading(list[0].ID); err != nil {
		t.Error(err)
	}

	if err := g.PauseClock(); err != nil {
		t.Fatal(err)
	}
	other := &Game{Root: root, pack: g.Pack(), log: g.log, session: new(g.Session())}
	if c := other.Clock(); !c.Paused || c.Since.IsZero() {
		t.Fatalf("another process does not see the clock stopped: %+v", c)
	}
	if err := other.ResumeClock(); err != nil {
		t.Fatal(err)
	}
	if s := g.Session(); clockPaused(s) || len(s.Pauses) != 1 || s.Pauses[0].To.IsZero() {
		t.Fatalf("after resuming: %+v", s.Pauses)
	}
	if err := g.update(func(s *play.Session) { s.Phase, s.DoneAt = play.Completed, time.Now().UTC() }); err != nil {
		t.Fatal(err)
	}
	if err := g.PauseClock(); !errors.Is(err, ErrClockNeedsGame) {
		t.Errorf("pausing a finished mission: err = %v", err)
	}
}
