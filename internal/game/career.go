package game

import (
	"path/filepath"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/career"
	"github.com/Bappoz/software-engineering-simulator/internal/evaluator"
	"github.com/Bappoz/software-engineering-simulator/internal/play"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/topology"
)

// Profile is the player's career over the whole campaign of this workspace:
// the finished missions of the history, plus the current one once it passed
// (ADR-0023).
func (g *Game) Profile() career.Profile {
	s := g.Session()
	hired := s.StartedAt
	if len(s.History) > 0 {
		hired = s.History[0].StartedAt
	}
	var missions []career.Mission
	var buys []career.Purchase
	for _, f := range s.History {
		missions = append(missions, g.careerMission(f.Scenario, f.ID, f.Title, f.DoneAt, f.Hints, f.Story, f.Cost, f.Budget))
		buys = append(buys, f.Purchases...)
	}
	if s.Phase == play.Completed {
		p := g.Pack()
		missions = append(missions, missionOf(p.Events, p.Manifest.Metadata.ID, p.Manifest.Metadata.Title, s.DoneAt, s.HintsShown, s.Story, s.PassCost, s.Budget))
	}
	buys = append(buys, s.Purchases...)
	return career.Compute(hired, missions, buys)
}

// careerMission reads a finished scenario's story events to tell which of its
// marks were emergencies and which were requests.
func (g *Game) careerMission(dir, id, title string, done time.Time, hints int, marks map[string]play.StoryMark, cost, budget float64) career.Mission {
	p, err := scenario.Load(dir)
	if err != nil && !filepath.IsAbs(dir) {
		p, err = scenario.Load(filepath.Join(g.Root, dir))
	}
	var events []scenario.StoryEvent
	if err == nil {
		events = p.Events
	} else {
		g.log.Warn("career: cannot read a finished scenario's story; no story bonus for it", "scenario", dir, "err", err)
	}
	return missionOf(events, id, title, done, hints, marks, cost, budget)
}

func missionOf(events []scenario.StoryEvent, id, title string, done time.Time, hints int, marks map[string]play.StoryMark, cost, budget float64) career.Mission {
	m := career.Mission{ID: id, Title: title, DoneAt: done, Hints: hints, Cost: cost, Budget: budget}
	for _, e := range events {
		mark := marks[e.ID]
		switch {
		case e.Kind == "emergencia" && !mark.MetAt.IsZero():
			m.EmergenciesMet++
		case e.Kind == "emergencia" && !mark.MissedAt.IsZero():
			m.EmergenciesMissed++
		case e.Kind == "pedido" && !mark.MetAt.IsZero():
			m.RequestsMet++
		}
	}
	return m
}

// NextHintPrice is what the next hint of this mission costs.
func (g *Game) NextHintPrice() int { return career.HintPrice(g.Session().HintsShown + 1) }

// NextLabPrice is what the next lab experiment of this mission costs.
func (g *Game) NextLabPrice() int { return career.LabPrice(g.Session().LabRuns + 1) }

// buy records a purchase in the session (s is being updated under the lock).
func buy(s *play.Session, item string, price int) {
	if price > 0 {
		s.Purchases = append(s.Purchases, career.Purchase{At: time.Now().UTC(), Item: item, Price: price})
	}
}

// costAtPass is the modeled monthly cost of a passing report and its budget.
func costAtPass(p *scenario.Pack, rep evaluator.Report) (cost, budget float64) {
	for _, o := range p.Manifest.Objectives {
		if o.Measure != topology.CostMeasure || o.Below == nil {
			continue
		}
		for _, r := range rep.Objectives {
			if r.ID == o.ID {
				return r.Median, *o.Below
			}
		}
	}
	return 0, 0
}
