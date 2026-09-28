package game

import (
	"errors"
	"log/slog"
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
	return profileOf(g.Root, g.Session(), g.Pack(), g.log)
}

// profileOf is the career of session s; current is its scenario's pack.
func profileOf(root string, s play.Session, current *scenario.Pack, log *slog.Logger) career.Profile {
	hired := s.StartedAt
	if len(s.History) > 0 {
		hired = s.History[0].StartedAt
	}
	var missions []career.Mission
	var buys []career.Purchase
	for _, f := range s.History {
		missions = append(missions, careerMission(root, log, f.Scenario, f.ID, f.Title, f.DoneAt, f.Hints, f.Story, f.Cost, f.Budget))
		buys = append(buys, f.Purchases...)
	}
	if s.Phase == play.Completed && current != nil {
		p := current
		missions = append(missions, missionOf(p, p.Manifest.Metadata.ID, p.Manifest.Metadata.Title, s.DoneAt, s.HintsShown, s.Story, s.PassCost, s.Budget))
	}
	buys = append(buys, s.Purchases...)
	return career.Compute(hired, missions, buys)
}

// careerMission reads a finished scenario's story events to tell which of its
// marks were emergencies and which were requests.
func careerMission(root string, log *slog.Logger, dir, id, title string, done time.Time, hints int, marks map[string]play.StoryMark, cost, budget float64) career.Mission {
	p, err := loadPack(root, dir)
	if err != nil {
		p = nil
		log.Warn("career: cannot read a finished scenario's story; no story bonus for it", "scenario", dir, "err", err)
	}
	return missionOf(p, id, title, done, hints, marks, cost, budget)
}

// missionOf is a finished mission as the career sees it; p (nil when it
// cannot be read) tells emergencies from requests, and whether it was a tutorial.
func missionOf(p *scenario.Pack, id, title string, done time.Time, hints int, marks map[string]play.StoryMark, cost, budget float64) career.Mission {
	m := career.Mission{ID: id, Title: title, DoneAt: done, Hints: hints, Cost: cost, Budget: budget}
	if p == nil {
		return m
	}
	m.Tutorial = len(p.Tutorial) > 0
	for _, e := range p.Events {
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
func (g *Game) NextHintPrice() int { return g.hintPrice(g.Session().HintsShown + 1) }

// hintPrice is career.HintPrice, except in a tutorial, where Tabby's help is free (ADR-0024).
func (g *Game) hintPrice(n int) int {
	if g.IsTutorial() {
		return 0
	}
	return career.HintPrice(n)
}

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

// loadPack reads a scenario pack named relative to the working directory or,
// failing that, to the repository root.
func loadPack(root, dir string) (*scenario.Pack, error) {
	p, err := scenario.Load(dir)
	if err != nil && !filepath.IsAbs(dir) {
		p, err = scenario.Load(filepath.Join(root, dir))
	}
	return p, err
}

// SaveSummary is a workspace as the save menu shows it: read from its session
// file alone, without Docker or a sandbox.
type SaveSummary struct {
	Empty    bool      `json:"empty"`
	Scenario string    `json:"scenario,omitempty"` // the current scenario's title
	Phase    string    `json:"phase,omitempty"`
	Missions int       `json:"missions"` // completed
	Level    string    `json:"level,omitempty"`
	Balance  int       `json:"balance"`
	PlayedAt time.Time `json:"played_at,omitzero"` // when the session was last saved
}

// Summarize reads the save in workspace; a workspace with no game is Empty.
func Summarize(root, workspace string) (SaveSummary, error) {
	s, fi, err := play.LoadSessionInfo(workspace)
	if errors.Is(err, play.ErrNoSession) {
		return SaveSummary{Empty: true}, nil
	}
	if err != nil {
		return SaveSummary{}, err
	}
	sum := SaveSummary{Phase: string(s.Phase), Missions: len(s.History), PlayedAt: fi.ModTime()}
	p, err := loadPack(root, s.Scenario)
	if err != nil {
		p = nil
		sum.Scenario = filepath.Base(s.Scenario)
	} else {
		sum.Scenario = p.Manifest.Metadata.Title
	}
	if s.Phase == play.Completed {
		sum.Missions++
	}
	prof := profileOf(root, *s, p, slog.New(slog.DiscardHandler))
	sum.Level, sum.Balance = prof.Level.Title, prof.Balance
	return sum, nil
}
