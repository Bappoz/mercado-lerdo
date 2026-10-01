package game

import (
	"strings"
	"testing"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/play"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

func TestEvalStory(t *testing.T) {
	t0 := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	at := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Minute) }
	calmTip := scenario.StoryEvent{ID: "tip", Kind: "pedido", From: "alvaro", Title: "t", Text: "x",
		At: &scenario.StoryAt{Phase: "calm", After: 2 * time.Minute}, Goal: "view_pulse", Met: "valeu"}
	live := scenario.StoryEvent{ID: "live", Kind: "emergencia", From: "alvaro", Title: "t", Text: "x",
		At: &scenario.StoryAt{Phase: "incident", After: time.Minute}, Goal: "test_pass", Within: 20 * time.Minute, Met: "salvou", Missed: "perdemos"}
	down := scenario.StoryEvent{ID: "down", Kind: "emergencia", From: "alvaro", Title: "t", Text: "x",
		At: &scenario.StoryAt{Phase: "incident"}, Goal: "store_up", Within: 5 * time.Minute, Met: "voltou", Missed: "demorou"}
	scold := scenario.StoryEvent{ID: "scold", Kind: "bronca", From: "alvaro", Title: "t", Text: "x", On: "test_failed"}
	calm := play.Session{Phase: play.Calm, StartedAt: t0}
	incident := play.Session{Phase: play.Incident, StartedAt: t0, IncidentAt: at(10)}
	never := func() bool { return false }
	always := func() bool { return true }

	for _, tc := range []struct {
		name  string
		ev    scenario.StoryEvent
		s     play.Session
		now   time.Time
		up    func() bool
		want  play.StoryMark
		quiet bool // nothing changes
	}{
		{name: "not yet", ev: calmTip, s: calm, now: at(1), up: never, quiet: true},
		{name: "fires on time", ev: calmTip, s: calm, now: at(3), up: never, want: play.StoryMark{FiredAt: at(2)}},
		{name: "a calm event nobody saw is backfilled", ev: calmTip, s: incident, now: at(30), up: never, want: play.StoryMark{FiredAt: at(2)}},
		{name: "a calm event past its phase never fires", ev: func() scenario.StoryEvent {
			e := calmTip
			e.At = &scenario.StoryAt{Phase: "calm", After: 15 * time.Minute}
			return e
		}(), s: incident, now: at(30), up: never, quiet: true},
		{name: "a goal met after it was asked", ev: calmTip,
			s: func() play.Session { s := calm; s.Viewed = map[string]time.Time{"pulse": at(4)}; return s }(), now: at(5), up: never,
			want: play.StoryMark{FiredAt: at(2), MetAt: at(4)}},
		{name: "a look before the request does not count", ev: calmTip,
			s: func() play.Session { s := calm; s.Viewed = map[string]time.Time{"pulse": at(1)}; return s }(), now: at(5), up: never,
			want: play.StoryMark{FiredAt: at(2)}},
		{name: "deadline met by a PASS", ev: live,
			s: func() play.Session { s := incident; s.Phase, s.DoneAt = play.Completed, at(25); return s }(), now: at(40), up: never,
			want: play.StoryMark{FiredAt: at(11), MetAt: at(25)}},
		{name: "a PASS after the deadline is still a miss", ev: live,
			s: func() play.Session { s := incident; s.Phase, s.DoneAt = play.Completed, at(35); return s }(), now: at(40), up: never,
			want: play.StoryMark{FiredAt: at(11), MissedAt: at(31)}},
		{name: "deadline still open", ev: live, s: incident, now: at(20), up: never, want: play.StoryMark{FiredAt: at(11)}},
		{name: "up before the crash is not back up", ev: down, s: incident, now: at(10), up: always, want: play.StoryMark{FiredAt: at(10)}},
		{name: "the store is back up in time", ev: down, s: func() play.Session { s := incident; s.CrashedAt = at(10); return s }(), now: at(12), up: always, want: play.StoryMark{FiredAt: at(10), MetAt: at(12)}},
		{name: "the store stayed down", ev: down, s: incident, now: at(16), up: never, want: play.StoryMark{FiredAt: at(10), MissedAt: at(15)}},
		{name: "the crash itself raises the alarm", ev: func() scenario.StoryEvent { e := down; e.At, e.On = nil, "store_down"; return e }(),
			s: func() play.Session { s := incident; s.CrashedAt = at(10).Add(20 * time.Second); return s }(), now: at(11), up: never,
			want: play.StoryMark{FiredAt: at(10).Add(20 * time.Second)}},
		{name: "a failed test scolds", ev: scold, s: func() play.Session { s := incident; s.LastFailAt = at(18); return s }(), now: at(19), up: never, want: play.StoryMark{FiredAt: at(18)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := evalStory([]scenario.StoryEvent{tc.ev}, tc.s, tc.now, tc.up)
			if tc.quiet {
				if got != nil {
					t.Fatalf("changed %+v, want nothing", got)
				}
				return
			}
			if got[tc.ev.ID] != tc.want {
				t.Errorf("mark = %+v, want %+v", got[tc.ev.ID], tc.want)
			}
		})
	}
}

func TestEvalStoryIsIdempotent(t *testing.T) {
	t0 := time.Now().Add(-time.Hour)
	ev := scenario.StoryEvent{ID: "e", Kind: "noticia", From: "otavio", Title: "t", Text: "x", At: &scenario.StoryAt{Phase: "calm"}}
	s := play.Session{Phase: play.Calm, StartedAt: t0}
	first := evalStory([]scenario.StoryEvent{ev}, s, time.Now(), func() bool { return false })
	s.Story = first
	if again := evalStory([]scenario.StoryEvent{ev}, s, time.Now(), func() bool { return false }); again != nil {
		t.Errorf("a fired event fired again: %+v", again)
	}
	v := storyView([]scenario.StoryEvent{ev}, s.Story, nil, nil)
	if len(v.Messages) != 1 || v.Messages[0].From != "Otávio (financeiro)" || v.Deadline != nil {
		t.Errorf("view = %+v", v)
	}
}

func TestStoryViewDeadline(t *testing.T) {
	t0 := time.Now()
	evs := []scenario.StoryEvent{
		{ID: "a", Kind: "emergencia", From: "alvaro", Title: "A", Text: "x", At: &scenario.StoryAt{Phase: "incident"}, Goal: "test_pass", Within: 30 * time.Minute, Met: "m", Missed: "n"},
		{ID: "b", Kind: "emergencia", From: "jessica", Title: "B", Text: "x", At: &scenario.StoryAt{Phase: "incident"}, Goal: "store_up", Within: 5 * time.Minute, Met: "m", Missed: "n"},
	}
	marks := map[string]play.StoryMark{"a": {FiredAt: t0}, "b": {FiredAt: t0}}
	if d := storyView(evs, marks, nil, nil).Deadline; d == nil || d.ID != "b" {
		t.Errorf("the most urgent deadline = %+v, want b", d)
	}
	marks["b"] = play.StoryMark{FiredAt: t0, MetAt: t0.Add(time.Minute)}
	v := storyView(evs, marks, nil, nil)
	if v.Deadline == nil || v.Deadline.ID != "a" || len(v.Messages) != 3 {
		t.Errorf("after b was met: %+v", v)
	}
}

func TestStoryReplyQuotesTheBaseline(t *testing.T) {
	t0 := time.Now()
	ev := scenario.StoryEvent{ID: "olhar", Kind: "pedido", From: "alvaro", Title: "T", Text: "x", At: &scenario.StoryAt{Phase: "calm"}, Goal: "view_pulse", Met: "Guarda esses números.\n\n{{baseline}}\n"}
	marks := map[string]play.StoryMark{"olhar": {FiredAt: t0, MetAt: t0}}
	b := &play.Baseline{At: t0, Signals: play.Signals{
		Endpoints: []play.Endpoint{{Name: "search", RPS: 12, ErrorRate: 0.001, P95s: 0.042}},
		CPU:       map[string]float64{"postgres": 0.22, "app": 0.08},
	}}
	reply := storyView([]scenario.StoryEvent{ev}, marks, b, nil).Messages[1].Text
	for _, want := range []string{"Guarda esses números.", "**search**: p95 42 ms · 12.0 req/s · 0.1% de erros", "app 8% · postgres 22%"} {
		if !strings.Contains(reply, want) {
			t.Errorf("reply %q lacks %q", reply, want)
		}
	}
	// Without a baseline the placeholder goes away, not the reply.
	if got := storyView([]scenario.StoryEvent{ev}, marks, nil, nil).Messages[1].Text; got != "Guarda esses números." {
		t.Errorf("reply without numbers = %q", got)
	}
}
