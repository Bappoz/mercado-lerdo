package game

import (
	"errors"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/play"
)

// The story's clock (ADR-0026): the story counts time from the phases'
// starts and from when a request was made (ADR-0022). The player may stop
// that clock to read; a pause is a stretch of wall time the story does not
// count. Only the story waits: production keeps receiving its traffic.

// ErrClockNeedsGame means there is no game whose clock could stop.
var ErrClockNeedsGame = errors.New("o relógio da história só existe com uma missão em andamento")

// clockAdd is the moment d of the story's time has passed since from: the
// pauses in the way push it. ok is false when an open pause is in the way,
// because that moment does not come until the clock runs again.
func clockAdd(pauses []play.Pause, from time.Time, d time.Duration) (at time.Time, ok bool) {
	at = from
	for _, p := range pauses {
		if !p.To.IsZero() && !p.To.After(at) {
			continue // behind
		}
		if gap := p.From.Sub(at); gap >= d {
			break
		} else if gap > 0 {
			d -= gap
		}
		if p.To.IsZero() {
			return time.Time{}, false
		}
		at = p.To
	}
	return at.Add(d), true
}

// frozen is pauses with the open one closed where it began: the clock as it
// stood when it stopped.
func frozen(pauses []play.Pause) []play.Pause {
	n := len(pauses)
	if n == 0 || !pauses[n-1].To.IsZero() {
		return pauses
	}
	out := append([]play.Pause(nil), pauses...)
	out[n-1].To = out[n-1].From
	return out
}

// clockPaused reports whether the story's clock is stopped.
func clockPaused(s play.Session) bool {
	n := len(s.Pauses)
	return n > 0 && s.Pauses[n-1].To.IsZero()
}

// pauseClock stops the clock at now; false when it was already stopped.
func pauseClock(s *play.Session, now time.Time) bool {
	if clockPaused(*s) {
		return false
	}
	s.Pauses = append(s.Pauses, play.Pause{From: now})
	return true
}

// resumeClock runs the clock again at now and returns how long it was
// stopped; false when it was running.
func resumeClock(s *play.Session, now time.Time) (time.Duration, bool) {
	if !clockPaused(*s) {
		return 0, false
	}
	p := &s.Pauses[len(s.Pauses)-1]
	p.To = now
	return p.To.Sub(p.From), true
}

// ClockView is the story's clock as the views show it.
type ClockView struct {
	Paused bool      `json:"paused"`
	Since  time.Time `json:"since,omitzero"` // when it stopped
}

// Clock is the story's clock now.
func (g *Game) Clock() ClockView {
	s := g.Session()
	if !clockPaused(s) {
		return ClockView{}
	}
	return ClockView{Paused: true, Since: s.Pauses[len(s.Pauses)-1].From}
}

// PauseClock stops the story's clock: no message arrives and no deadline
// runs until ResumeClock. It is for reading; any action on production
// (deploy, check, test, lab, starting the day) runs it again.
func (g *Game) PauseClock() error {
	if s := g.Session(); s.Phase != play.Calm && s.Phase != play.Incident {
		return ErrClockNeedsGame
	}
	paused := false
	if err := g.update(func(s *play.Session) { paused = pauseClock(s, time.Now().UTC()) }); err != nil {
		return err
	}
	if paused {
		g.Event("clock_paused", map[string]any{"phase": string(g.Session().Phase)})
	}
	return nil
}

// ResumeClock runs the story's clock again; a no-op when it was running.
func (g *Game) ResumeClock() error {
	var stopped time.Duration
	resumed := false
	if err := g.update(func(s *play.Session) { stopped, resumed = resumeClock(s, time.Now().UTC()) }); err != nil {
		return err
	}
	if resumed {
		g.Event("clock_resumed", map[string]any{"seconds": int(stopped.Seconds())})
	}
	return nil
}
