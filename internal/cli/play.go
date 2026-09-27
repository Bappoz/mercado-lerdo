package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/game"
	"github.com/Bappoz/software-engineering-simulator/internal/play"
)

const defaultWorkspace = "workspace"

// gameFlags are the flags every player command shares.
type gameFlags struct {
	root, scenario, workspace string
}

func (f *gameFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.root, "root", ".", "repository root")
	fs.StringVar(&f.scenario, "scenario", "", "scenario pack directory (default: the workspace's current scenario, or C1 for a new game)")
	fs.StringVar(&f.workspace, "workspace", defaultWorkspace, "your workspace (a copy of the app you edit)")
}

func (f gameFlags) open(ctx context.Context, log *slog.Logger) (*game.Game, error) {
	return game.Open(ctx, game.Options{Root: f.root, Scenario: f.scenario, Workspace: f.workspace, Log: log})
}

// terminal is one player command in the terminal.
type terminal struct {
	g      *game.Game
	stdout io.Writer
	stderr io.Writer
}

// openGame parses the common flags and opens the game. With needSession, a
// missing session is an error.
func openGame(ctx context.Context, name string, args []string, stdout, stderr io.Writer, needSession bool) (*terminal, int) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var f gameFlags
	f.register(fs)
	if err := fs.Parse(args); err != nil {
		return nil, 2
	}
	log := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	g, err := f.open(ctx, log)
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return nil, 1
	}
	if needSession && !g.HasSession() {
		g.Close()
		fmt.Fprintln(stderr, "sesim: nenhuma partida neste workspace. Comece com: sesim play")
		return nil, 1
	}
	return &terminal{g: g, stdout: stdout, stderr: stderr}, 0
}

func (t *terminal) fail(err error) int {
	if errors.Is(err, game.ErrNotRunning) {
		err = fmt.Errorf("%w. Rode `sesim play` para subir de novo", err)
	}
	fmt.Fprintln(t.stderr, "sesim:", err)
	return 1
}

func (t *terminal) say(line string) { fmt.Fprintln(t.stdout, line) }

func (t *terminal) printDoc(ctx context.Context, rel string) error {
	md, err := t.g.Document(ctx, rel, play.CLIWords)
	if err != nil {
		return err
	}
	fmt.Fprintln(t.stdout, md)
	return nil
}

// runPlay: start a new game, or resume the one in the workspace.
func runPlay(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "play", args, stdout, stderr, false)
	if t == nil {
		return code
	}
	defer t.g.Close()
	if _, err := t.g.Start(ctx, t.say); err != nil {
		return t.fail(err)
	}
	if t.g.Session().Phase == play.Completed {
		fmt.Fprintln(stdout, "Este cenário já foi concluído. O debrief está em:", filepath.Join(t.g.Pack().Dir, t.g.Pack().Manifest.Player.Debrief))
	}
	fmt.Fprintln(stdout)
	if err := t.printDoc(ctx, t.g.Phase().Briefing); err != nil {
		return t.fail(err)
	}
	t.tellStory(ctx)
	return 0
}

// runNext: end the calm phase; the growth event starts the incident.
func runNext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "next", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	started, err := t.g.Next(ctx)
	if err != nil {
		return t.fail(err)
	}
	if !started {
		fmt.Fprintln(stdout, "Nada a avançar: o incidente já começou. `sesim status` mostra como está a produção.")
		return 0
	}
	if err := t.printDoc(ctx, t.g.Phase().Briefing); err != nil {
		return t.fail(err)
	}
	return 0
}

// tellStory prints what the story said since it was last advanced, and the
// open deadline (ADR-0022). Commands call it last, after their own output.
func (t *terminal) tellStory(ctx context.Context) {
	v, err := t.g.Story(ctx)
	if err != nil {
		fmt.Fprintln(t.stderr, "sesim: story:", err)
		return
	}
	for _, m := range v.Messages {
		if m.New {
			printStoryMessage(t.stdout, m)
		}
	}
	printDeadline(t.stdout, v.Deadline)
}

func printStoryMessage(w io.Writer, m game.StoryMessage) {
	fmt.Fprintf(w, "\n── %s · %s · %s\n%s\n", m.At.Local().Format("15:04"), m.From, m.Title, strings.TrimSpace(play.RenderBriefing(m.Text, play.CLIWords)))
}

func printDeadline(w io.Writer, d *game.Deadline) {
	if d == nil {
		return
	}
	left := time.Until(d.Until).Round(time.Minute)
	fmt.Fprintf(w, "\n⏱  %s (%s): até %s, faltam %s. `sesim inbox` mostra o pedido.\n", d.Title, d.From, d.Until.Local().Format("15:04"), left)
}

// runInbox: every message of the story so far, and the open deadline.
func runInbox(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "inbox", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	v, err := t.g.Story(ctx)
	if err != nil {
		return t.fail(err)
	}
	if len(v.Messages) == 0 {
		fmt.Fprintln(stdout, "Nenhuma mensagem do pessoal ainda.")
	}
	for _, m := range v.Messages {
		printStoryMessage(stdout, m)
	}
	printDeadline(stdout, v.Deadline)
	return 0
}

// runStatus: production signals right now.
func runStatus(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "status", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	if !t.g.Running() {
		return t.fail(game.ErrNotRunning)
	}
	fmt.Fprintln(stdout, "Medindo por 10 s…")
	sig, err := t.g.Signals(ctx, 10*time.Second)
	if err != nil {
		return t.fail(err)
	}
	if len(sig.Endpoints) == 0 {
		fmt.Fprintln(stdout, "Ainda sem dados de tráfego (ele começa alguns segundos depois do `sesim play`/`sesim next`). Tente de novo.")
	}
	sig.Write(stdout, nil)
	t.g.Event("status_viewed", map[string]any{"phase": string(t.g.Session().Phase)})
	if err := t.g.MarkViewed("pulse"); err != nil {
		return t.fail(err)
	}
	t.tellStory(ctx)
	return 0
}

// runCheck: Quick Check — apply the workspace and measure the live traffic (indicative).
func runCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "check", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	defer t.tellStory(ctx) // after the command's own output
	res, err := t.g.Check(ctx, t.say)
	if err != nil {
		return t.fail(err)
	}
	fmt.Fprintf(stdout, "\nQUICK CHECK (indicativo: uma medição curta, sem reset de dados; o que vale é `sesim test`)\n\n")
	res.Now.Write(stdout, res.Before)
	fmt.Fprintf(stdout, "\n(%s no total)\n", res.Elapsed.Round(time.Second))
	return 0
}

// runHint: the next rung of the hint ladder.
func runHint(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "hint", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	h, ok, err := t.g.Hint()
	if err != nil {
		return t.fail(err)
	}
	if !ok {
		fmt.Fprintln(stdout, "Não há mais dicas. As anteriores continuam valendo; releia com calma.")
		return 0
	}
	fmt.Fprintf(stdout, "Dica %d de %d · %s\n\n%s\n", h.Level, len(t.g.Pack().Hints), h.Kind, play.RenderBriefing(h.Text, play.CLIWords))
	t.tellStory(ctx)
	return 0
}

// runShell: interactive shell inside the Nexora network.
func runShell(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "shell", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	if !t.g.Running() {
		return t.fail(game.ErrNotRunning)
	}
	t.g.Event("shell_opened", nil)
	fmt.Fprintln(stdout, "Terminal na rede da loja (psql já conectado ao banco). `exit` para sair.")
	if err := t.g.Shell(ctx, os.Stdin, stdout, stderr); err != nil {
		return t.fail(err)
	}
	return 0
}

// runStop: stop the background traffic; with --down also destroy the sandbox
// (the workspace and the progress stay; `sesim play` brings a fresh one up).
func runStop(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	down := slices.Contains(args, "--down")
	args = slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == "--down" })
	t, code := openGame(ctx, "stop", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	if err := t.g.Stop(ctx, down); err != nil {
		return t.fail(err)
	}
	if !down {
		fmt.Fprintln(stdout, "Tráfego parado; a loja continua de pé. `sesim play` retoma; `sesim stop --down` desliga tudo.")
		return 0
	}
	fmt.Fprintln(stdout, "Loja desligada (nada ficou para trás). Seu workspace e o progresso continuam; `sesim play` sobe de novo.")
	return 0
}
