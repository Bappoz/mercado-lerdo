package game

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/campaign"
	"github.com/Bappoz/software-engineering-simulator/internal/events"
	"github.com/Bappoz/software-engineering-simulator/internal/play"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/topology"
	"github.com/Bappoz/software-engineering-simulator/internal/workspace"
)

// Campaign errors a view shows to the player.
var (
	ErrNotInCampaign = errors.New("este cenário não faz parte de uma campanha")
	ErrNotCompleted  = errors.New("conclua o cenário atual (PASS no Production Test) antes de seguir")
	ErrStageDone     = errors.New("não há próximo cenário: o stage terminou")
)

// Mission is one scenario of the campaign as the mission board shows it.
type Mission struct {
	Dir    string         `json:"dir"`
	ID     string         `json:"id"`
	Title  string         `json:"title"`
	State  string         `json:"state"` // done | current | locked
	Result *play.Finished `json:"result,omitempty"`
}

// CampaignView is the mission board.
type CampaignView struct {
	Stage      string    `json:"stage"`
	Missions   []Mission `json:"missions"`
	CanAdvance bool      `json:"can_advance"`
}

// Campaign describes the stage this game's scenario belongs to.
func (g *Game) Campaign() (CampaignView, error) {
	st, err := campaign.StageOf(g.Pack().Dir)
	if err != nil {
		return CampaignView{}, ErrNotInCampaign
	}
	cur, _ := st.Index(g.Pack().Dir)
	s := g.Session()
	done := map[string]play.Finished{}
	for _, f := range s.History {
		done[f.ID] = f
	}
	v := CampaignView{Stage: st.Metadata.Title}
	for i := range st.Scenarios {
		p, err := scenario.Load(st.Path(i))
		if err != nil {
			return CampaignView{}, err
		}
		m := Mission{Dir: st.Path(i), ID: p.Manifest.Metadata.ID, Title: p.Manifest.Metadata.Title, State: "locked"}
		if f, ok := done[m.ID]; ok {
			m.State, m.Result = "done", &f
		}
		if i == cur {
			m.State = "current"
			if s.Phase == play.Completed {
				m.State = "done"
				m.Result = &play.Finished{ID: m.ID, Title: m.Title, StartedAt: s.StartedAt, DoneAt: s.DoneAt,
					Checks: s.Checks, Tests: s.Tests, Hints: s.HintsShown, Canonical: s.Canonical}
			}
		}
		v.Missions = append(v.Missions, m)
	}
	v.CanAdvance = s.Phase == play.Completed && cur+1 < len(st.Scenarios)
	return v, nil
}

// Advance moves a completed game to the next scenario of its stage. The
// workspace keeps the player's code, changed only by the next scenario's
// story, unless canonical: then it becomes that scenario's canonical start
// (the player's code stays in the git history). The sandbox goes down: the
// next scenario's world is different; `play` brings it up again.
func (g *Game) Advance(ctx context.Context, canonical bool, say Say) (*scenario.Pack, error) {
	st, err := campaign.StageOf(g.Pack().Dir)
	if err != nil {
		return nil, ErrNotInCampaign
	}
	cur, _ := st.Index(g.Pack().Dir)
	s := g.Session()
	if s.Phase != play.Completed {
		return nil, ErrNotCompleted
	}
	if cur+1 >= len(st.Scenarios) {
		return nil, ErrStageDone
	}
	next, err := scenario.Load(st.Path(cur + 1))
	if err != nil {
		return nil, err
	}
	ws := s.Workspace
	if err := gitCommitAll(ctx, ws, "Fim do cenário "+g.Pack().Manifest.Metadata.Title); err != nil {
		return nil, err
	}
	if st := g.running(); st != nil {
		say("Desligando a Nexora deste cenário…")
		if err := g.Stop(ctx, true); err != nil {
			return nil, err
		}
	}
	how := "a história do próximo cenário"
	if canonical {
		how = "o estado canônico"
		say("Trocando o workspace pelo estado canônico de " + next.Manifest.Metadata.Title + " (o seu código fica no histórico do git)…")
		if err := g.adoptCanonical(ctx, next); err != nil {
			return nil, err
		}
	} else {
		say("Aplicando a história de " + next.Manifest.Metadata.Title + " ao seu workspace…")
		if story := st.Scenarios[cur+1].Story; len(story) > 0 {
			if err := topology.SetFile(ws, story); err != nil {
				return nil, fmt.Errorf("a história não coube no seu topology.yaml: %w (use o estado canônico)", err)
			}
		}
	}
	tag := "sesim/scenario-start/" + filepath.Base(next.Dir)
	if err := gitCommitAll(ctx, ws, fmt.Sprintf("Início do cenário %s (%s)", next.Manifest.Metadata.Title, how)); err != nil {
		return nil, err
	}
	if err := runGit(ctx, ws, "tag", "-f", tag); err != nil {
		return nil, err
	}
	finished := play.Finished{Scenario: s.Scenario, ID: g.Pack().Manifest.Metadata.ID, Title: g.Pack().Manifest.Metadata.Title,
		StartedAt: s.StartedAt, DoneAt: s.DoneAt, Checks: s.Checks, Tests: s.Tests, Hints: s.HintsShown, Canonical: s.Canonical}
	from := g.Pack().Manifest.Metadata.ID
	if err := g.update(func(s *play.Session) {
		*s = play.Session{
			Scenario: next.Dir, Workspace: s.Workspace, Phase: play.Calm, StartedAt: time.Now().UTC(),
			Canonical: canonical, History: append(s.History, finished),
		}
	}); err != nil {
		return nil, err
	}
	_ = os.Remove(lastTestPath(ws))
	g.mu.Lock()
	g.pack, g.lastTest = next, nil
	g.events = events.Log{Path: play.EventsPath(ws), Scenario: next.Manifest.Metadata.ID}
	g.mu.Unlock()
	g.Event("scenario_advanced", map[string]any{"from": from, "canonical": canonical})
	return next, nil
}

// adoptCanonical replaces the workspace's files with next's canonical start,
// keeping .git (the player's history) and .sesim (the game's state).
func (g *Game) adoptCanonical(ctx context.Context, next *scenario.Pack) error {
	ws := g.Session().Workspace
	base := filepath.Join(g.Root, next.Manifest.Player.WorkspaceBase)
	dir, cleanup, err := workspace.Materialize(ctx, base, next.StartPatches()...)
	if err != nil {
		return err
	}
	defer cleanup()
	entries, err := os.ReadDir(ws)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == ".git" || e.Name() == play.StateDirName || e.Name() == ".gitignore" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(ws, e.Name())); err != nil {
			return err
		}
	}
	return os.CopyFS(ws, os.DirFS(dir))
}
