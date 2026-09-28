package game

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/career"

	"github.com/Bappoz/software-engineering-simulator/internal/play"
	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
	"github.com/Bappoz/software-engineering-simulator/internal/topology"
)

const root = "../.."

// newTestGame is a game without a sandbox: enough for the campaign and git,
// which never touch Docker unless a sandbox is running.
func newTestGame(t *testing.T) *Game {
	const scenarioDir = "scenarios/stage1/c1-slow-search"
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	p, err := scenario.Load(filepath.Join(root, scenarioDir))
	if err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(t.TempDir(), "ws")
	ctx := context.Background()
	if err := play.CreateWorkspace(ctx, filepath.Join(root, p.Manifest.Player.WorkspaceBase), ws,
		"sesim/scenario-start/"+filepath.Base(p.Dir), p.StartPatches()...); err != nil {
		t.Fatal(err)
	}
	return &Game{Root: root, pack: p, log: slog.New(slog.DiscardHandler),
		session: &play.Session{Scenario: p.Dir, Workspace: ws, Phase: play.Calm, StartedAt: time.Now().UTC()}}
}

func TestCampaignCarriesTheWorkspaceForward(t *testing.T) {
	g := newTestGame(t)
	ctx := context.Background()
	ws := g.Session().Workspace
	say := func(string) {}

	if _, err := g.Advance(ctx, false, say); !errors.Is(err, ErrNotCompleted) {
		t.Fatalf("advance before PASS: err = %v", err)
	}
	// The player's own C1 fix: a migration of theirs.
	if err := os.WriteFile(filepath.Join(ws, "migrations", "0002_mine.sql"), []byte("SELECT 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	complete := func() {
		if err := g.update(func(s *play.Session) { s.Phase, s.DoneAt, s.Tests = play.Completed, time.Now().UTC(), 1 }); err != nil {
			t.Fatal(err)
		}
	}
	complete()
	v, err := g.Campaign()
	if err != nil || !v.CanAdvance || v.Missions[0].State != "done" || v.Missions[1].State != "locked" {
		t.Fatalf("board before advancing = %+v, %v", v, err)
	}

	next, err := g.Advance(ctx, false, say)
	if err != nil {
		t.Fatal(err)
	}
	if next.Manifest.Metadata.ID != "stage1/c2-connection-pressure" || g.Pack().Manifest.Metadata.ID != next.Manifest.Metadata.ID {
		t.Fatalf("advanced to %s", g.Pack().Manifest.Metadata.ID)
	}
	if _, err := os.Stat(filepath.Join(ws, "migrations", "0002_mine.sql")); err != nil {
		t.Error("the player's own code must carry over")
	}
	top, err := topology.Load(ws)
	if err != nil || top.Components.App.Replicas != 3 || top.Components.App.DBMaxConns != 20 {
		t.Errorf("C2's story not applied: %+v, %v", top.Components.App, err)
	}
	s := g.Session()
	if s.Phase != play.Calm || len(s.History) != 1 || s.History[0].ID != "stage1/c1-slow-search" || s.Tests != 0 {
		t.Errorf("session after advancing = %+v", s)
	}
	reloaded, err := play.LoadSession(ws)
	if err != nil || reloaded.Scenario != next.Dir {
		t.Errorf("saved session = %+v, %v", reloaded, err)
	}
	log, err := g.GitLog(ctx, 5)
	if err != nil || !slices.ContainsFunc(log, func(c Commit) bool { return slices.Contains(c.Refs, "sesim/scenario-start/c2-connection-pressure") }) {
		t.Errorf("log lacks C2's start tag: %+v, %v", log, err)
	}

	// C2 done; enter C3 from its canonical start.
	complete()
	if _, err := g.Advance(ctx, true, say); err != nil {
		t.Fatal(err)
	}
	orders, err := os.ReadFile(filepath.Join(ws, "internal", "store", "orders.go"))
	if err != nil || !strings.Contains(string(orders), "reserve tira o estoque") {
		t.Error("the canonical C3 start has C2's reference fix")
	}
	if _, err := os.Stat(filepath.Join(ws, "migrations", "0002_mine.sql")); !os.IsNotExist(err) {
		t.Error("the canonical start replaces the player's files")
	}
	if _, err := os.Stat(filepath.Join(ws, ".sesim", "state.json")); err != nil {
		t.Error("the game state must survive the swap")
	}
	if top, _ := topology.Load(ws); top.Components.App.Replicas != 1 {
		t.Errorf("C3 starts on one instance, got %d", top.Components.App.Replicas)
	}
	if out, err := gitOutput(ctx, ws, "log", "--all", "--format=%s"); err != nil || !strings.Contains(out, "Fim do cenário Pressão de Conexões") {
		t.Errorf("the player's C2 code must stay in history: %q, %v", out, err)
	}
	if !g.Session().Canonical || len(g.Session().History) != 2 {
		t.Errorf("session = %+v", g.Session())
	}

	complete()
	if _, err := g.Advance(ctx, false, say); !errors.Is(err, ErrStageDone) {
		t.Errorf("after the last scenario: err = %v", err)
	}
}

func TestGit(t *testing.T) {
	g := newTestGame(t)
	ctx := context.Background()
	ws := g.Session().Workspace
	if c, err := g.GitStatus(ctx); err != nil || len(c) != 0 {
		t.Fatalf("clean workspace status = %+v, %v", c, err)
	}
	if _, err := g.GitCommit(ctx, "nothing"); !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("empty commit err = %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "API.md"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "migrations", "0002_idx.sql"), []byte("CREATE INDEX x ON y (z);\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := g.GitStatus(ctx)
	if err != nil || !slices.Equal(c, []FileChange{{"API.md", "modified"}, {"migrations/0002_idx.sql", "untracked"}}) {
		t.Fatalf("status = %+v, %v", c, err)
	}
	if d, err := g.GitDiff(ctx, "migrations/0002_idx.sql", false); err != nil || !strings.Contains(d, "+CREATE INDEX") {
		t.Errorf("untracked diff = %q, %v", d, err)
	}
	for _, since := range []bool{false, true} {
		if d, err := g.GitDiff(ctx, "", since); err != nil || !strings.Contains(d, "--- a/API.md") || !strings.Contains(d, "+CREATE INDEX") {
			t.Errorf("whole diff (since start: %v) must include new files: %q, %v", since, d, err)
		}
	}
	if d, err := g.GitDiff(ctx, "API.md", false); err != nil || !strings.Contains(d, "+changed") {
		t.Errorf("diff = %q, %v", d, err)
	}
	if _, err := g.GitDiff(ctx, "../../etc/passwd", false); err == nil {
		t.Error("a path outside git status must be refused")
	}
	if _, err := g.GitCommit(ctx, "  "); err == nil {
		t.Error("an empty message must be refused")
	}
	commit, err := g.GitCommit(ctx, "Índice para a busca")
	if err != nil || commit.Subject != "Índice para a busca" {
		t.Fatalf("commit = %+v, %v", commit, err)
	}
	if d, err := g.GitDiff(ctx, "", true); err != nil || !strings.Contains(d, "0002_idx.sql") {
		t.Errorf("diff since the scenario start = %q, %v", d, err)
	}
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("outside the workspace\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(ws, "leak")); err != nil {
		t.Fatal(err)
	}
	if d, err := g.GitDiff(ctx, "leak", false); err != nil || strings.Contains(d, "outside") || !strings.Contains(d, "+"+secret) {
		t.Errorf("an untracked link must show its target, not what it points to: %q, %v", d, err)
	}
}

// The UI and a CLI command can hold the same workspace at once: neither may
// save over what the other saved.
func TestSessionFollowsOtherProcesses(t *testing.T) {
	ui := newTestGame(t)
	cli := &Game{Root: root, pack: ui.Pack(), log: ui.log, session: new(ui.Session())}
	ctx := context.Background()

	if err := cli.update(func(s *play.Session) { s.Phase = play.Completed; s.Tests++ }); err != nil {
		t.Fatal(err)
	}
	if err := ui.update(func(s *play.Session) { s.HintsShown++ }); err != nil {
		t.Fatal(err)
	}
	if s := cli.Session(); s.Phase != play.Completed || s.Tests != 1 || s.HintsShown != 1 {
		t.Fatalf("a stale process saved over another's progress: %+v", s)
	}
	if _, err := cli.Advance(ctx, false, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if got := ui.Pack().Manifest.Metadata.ID; got != "stage1/c2-connection-pressure" {
		t.Errorf("the UI still plays %s after the CLI advanced", got)
	}
	if h := ui.Session().History; len(h) != 1 || h[0].Tests != 1 {
		t.Errorf("history = %+v", h)
	}
}

// The career follows the campaign: what one mission earned and bought is
// still there after moving to the next (ADR-0023).
func TestProfileFollowsTheCampaign(t *testing.T) {
	g := newTestGame(t)
	ctx := context.Background()
	if p := g.Profile(); p.Balance != career.SigningBonus || p.Level.Title != "Júnior I" || len(p.Payslips) != 0 {
		t.Fatalf("first day: %+v", p)
	}
	// C1 is the tutorial (ADR-0024): every hint is free.
	for range 2 {
		if _, ok, err := g.Hint(); !ok || err != nil {
			t.Fatalf("hint: %v %v", ok, err)
		}
	}
	if p := g.Profile(); p.Balance != career.SigningBonus || g.NextHintPrice() != 0 {
		t.Errorf("after two hints in the tutorial: balance %d, next hint %d", p.Balance, g.NextHintPrice())
	}
	// The emergency of C1 met, then the PASS.
	now := time.Now().UTC()
	if err := g.update(func(s *play.Session) {
		s.Story = map[string]play.StoryMark{"live-da-lu": {FiredAt: now.Add(-time.Hour), MetAt: now}}
		s.Phase, s.DoneAt, s.PassCost, s.Budget = play.Completed, now, 70, 100
	}); err != nil {
		t.Fatal(err)
	}
	p := g.Profile()
	if len(p.Payslips) != 1 {
		t.Fatalf("payslips %+v", p.Payslips)
	}
	// Júnior I, 4000: +20 % emergency, +15 % no hints (the tutorial's are free), +10 % savings.
	if got := p.Payslips[0].Total; got != 4000+800+600+400 {
		t.Errorf("C1 paid %d", got)
	}
	if _, err := g.Advance(ctx, false, func(string) {}); err != nil {
		t.Fatal(err)
	}
	after := g.Profile()
	if after.Balance != p.Balance || after.XP != p.XP || len(after.Payslips) != 1 {
		t.Errorf("the career changed on advancing: before %+v, after %+v", p, after)
	}
	if g.IsTutorial() || g.NextHintPrice() != 0 || g.hintPrice(2) != career.HintPrice(2) {
		t.Error("after the tutorial, the first hint is free and the next ones cost again")
	}
}
