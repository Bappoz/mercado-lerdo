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
	"strconv"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/career"
	"github.com/Bappoz/software-engineering-simulator/internal/game"
	"github.com/Bappoz/software-engineering-simulator/internal/play"
)

const defaultWorkspace = "workspace"

// gameFlags are the flags every player command shares.
type gameFlags struct {
	root, scenario, workspace string
}

func (f *gameFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.root, "root", ".", "raiz do repositório do jogo")
	fs.StringVar(&f.scenario, "scenario", "", "diretório do pacote do cenário (padrão: o cenário atual do workspace, ou o C1 numa partida nova)")
	fs.StringVar(&f.workspace, "workspace", defaultWorkspace, "o seu workspace (a cópia do app que você edita)")
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
	ptFlags(fs, stderr)
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
	t.tellStory(ctx)
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
	printDeadline(t.stdout, v.Deadline, t.g.Clock())
	t.tellTutorial(ctx)
}

// tellTutorial prints Slotthy's step when it has just begun (ADR-0024);
// `sesim slotthy` prints it at any time.
func (t *terminal) tellTutorial(ctx context.Context) {
	v, err := t.g.Tutorial(ctx, play.CLIWords, true)
	if err != nil {
		fmt.Fprintln(t.stderr, "sesim: treinamento:", err)
		return
	}
	if v != nil && v.New {
		printTutorial(t.stdout, v)
	}
}

func printTutorial(w io.Writer, v *game.TutorialView) {
	if v.Done {
		fmt.Fprintf(w, "\n── Slotthy · treinamento concluído\n")
		if v.Before != "" {
			fmt.Fprintln(w, v.Before)
		}
		return
	}
	if v.Step == "" {
		fmt.Fprintln(w, "\n── Slotthy · treinamento: o próximo passo começa com o incidente.")
		return
	}
	fmt.Fprintf(w, "\n── Slotthy · treinamento, passo %d de %d\n", v.Index, v.Total)
	if v.Before != "" {
		fmt.Fprintf(w, "%s\n\n", v.Before)
	}
	fmt.Fprintln(w, v.Text)
	for _, c := range v.Choices {
		fmt.Fprintf(w, "  %s) %s\n", c.ID, c.Text)
	}
	if v.Wrong != "" {
		fmt.Fprintf(w, "\n%s\n", v.Wrong)
	}
	if v.Help != "" {
		fmt.Fprintf(w, "\nAjuda da Slotthy: %s\n", v.Help)
	}
	switch {
	case len(v.Choices) > 0:
		fmt.Fprintln(w, "\nResponda com `sesim slotthy responder <letra>`.")
	case v.Continue:
		fmt.Fprintln(w, "\nPara avisar a Slotthy: `sesim slotthy ok`.")
	}
	if v.CanHelp {
		fmt.Fprintln(w, "Travou? `sesim slotthy ajuda`.")
	}
	if len(v.Past) > 0 {
		fmt.Fprintln(w, "Para rever um passo anterior: `sesim slotthy passo`.")
	}
}

// runSlotthy: what Slotthy says in the guided mode, and the player's answers to her.
func runSlotthy(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	action, choice := "", ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		action, args = args[0], args[1:]
	}
	if action == "responder" {
		if len(args) == 0 || strings.HasPrefix(args[0], "-") {
			fmt.Fprintln(stderr, "uso: sesim slotthy responder <letra>")
			return 2
		}
		choice, args = args[0], args[1:]
	}
	past := 0 // `passo <n>`: the step to read again; 0 lists them
	if action == "passo" && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 {
			fmt.Fprintln(stderr, "uso: sesim slotthy passo [número]")
			return 2
		}
		past, args = n, args[1:]
	}
	if !slices.Contains([]string{"", "ok", "ajuda", "responder", "passo"}, action) {
		fmt.Fprintln(stderr, "uso: sesim slotthy [ok | ajuda | responder <letra> | passo [número]]")
		return 2
	}
	t, code := openGame(ctx, "slotthy", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	var err error
	switch action {
	case "ok":
		err = t.g.TutorialOK(ctx)
	case "ajuda":
		err = t.g.TutorialHelp(ctx)
	case "responder":
		var right bool
		if right, err = t.g.TutorialAnswer(ctx, choice); err == nil && right {
			fmt.Fprintln(stdout, "Isso!")
		}
	}
	if err != nil {
		return t.fail(err)
	}
	v, err := t.g.Tutorial(ctx, play.CLIWords, true)
	if err != nil {
		return t.fail(err)
	}
	if v == nil {
		fmt.Fprintln(stdout, "Este cenário não tem treinamento. Se travar, `sesim hint` dá uma dica por vez.")
		return 0
	}
	if action == "passo" {
		if err := printPastSteps(stdout, v, past); err != nil {
			return t.fail(err)
		}
		return 0
	}
	printTutorial(stdout, v)
	return 0
}

// printPastSteps prints the step n again, or lists the steps behind when n is 0.
func printPastSteps(w io.Writer, v *game.TutorialView, n int) error {
	if len(v.Past) == 0 {
		fmt.Fprintln(w, "Ainda não há passos para rever: o treinamento está no primeiro.")
		return nil
	}
	if n == 0 {
		fmt.Fprintln(w, "\n── Slotthy · passos do treinamento que já ficaram para trás")
		for _, p := range v.Past {
			line, _, _ := strings.Cut(p.Text, "\n")
			fmt.Fprintf(w, "  %2d  %s\n", p.Index, line)
		}
		fmt.Fprintln(w, "\nPara ler um deles de novo: `sesim slotthy passo <número>`.")
		return nil
	}
	i := slices.IndexFunc(v.Past, func(p game.PastStep) bool { return p.Index == n })
	if i < 0 {
		return fmt.Errorf("o passo %d ainda não ficou para trás; `sesim slotthy passo` lista os que dá para rever", n)
	}
	p := v.Past[i]
	fmt.Fprintf(w, "\n── Slotthy · treinamento, passo %d de %d (revendo)\n%s\n", p.Index, v.Total, p.Text)
	if p.Done != "" {
		fmt.Fprintf(w, "\n%s\n", p.Done)
	}
	return nil
}

func printStoryMessage(w io.Writer, m game.StoryMessage) {
	fmt.Fprintf(w, "\n── %s · %s · %s\n%s\n", m.At.Local().Format("15:04"), m.From, m.Title, strings.TrimSpace(play.RenderBriefing(m.Text, play.CLIWords)))
}

// printDeadline prints the open deadline; with the story's clock stopped
// (ADR-0026) it says how much time is left when it runs again.
func printDeadline(w io.Writer, d *game.Deadline, clock game.ClockView) {
	if clock.Paused {
		fmt.Fprintln(w, "\n⏸  O relógio da história está parado: nenhuma mensagem chega e nenhum prazo corre. `sesim clock resume` retoma.")
	}
	if d == nil {
		return
	}
	if clock.Paused {
		left := d.Until.Sub(clock.Since).Round(time.Minute)
		fmt.Fprintf(w, "⏱  %s (%s): faltam %s quando o relógio voltar. `sesim inbox` mostra o pedido.\n", d.Title, d.From, left)
		return
	}
	left := time.Until(d.Until).Round(time.Minute)
	fmt.Fprintf(w, "\n⏱  %s (%s): até %s, faltam %s. `sesim inbox` mostra o pedido.\n", d.Title, d.From, d.Until.Local().Format("15:04"), left)
}

// runReading: the documentation Slotthy recommends for the mission (ADR-0026).
func runReading(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "reading", args, stdout, stderr, false)
	if t == nil {
		return code
	}
	defer t.g.Close()
	list := t.g.Reading(ctx, play.CLIWords)
	if len(list) == 0 {
		fmt.Fprintln(stdout, "A Slotthy não separou leituras para esta missão.")
		return 0
	}
	fmt.Fprintln(stdout, "── Slotthy · leituras para esta missão (nenhuma é obrigatória)")
	for i, r := range list {
		lang := map[string]string{"pt": "em português", "en": "em inglês"}[r.Lang]
		fmt.Fprintf(stdout, "\n%d. %s\n   %s · ≈ %d min · %s\n   %s\n", i+1, r.Title, r.Source, r.Minutes, lang, r.URL)
		for line := range strings.SplitSeq(r.Why, "\n") {
			fmt.Fprintf(stdout, "   %s\n", line)
		}
	}
	fmt.Fprintln(stdout, "\nPara ler sem perder prazo: `sesim clock pause` para o relógio da história; `sesim clock resume` retoma.")
	return 0
}

// runClock: stop or run the story's clock (ADR-0026).
func runClock(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	action := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		action, args = args[0], args[1:]
	}
	if !slices.Contains([]string{"", "pause", "resume"}, action) {
		fmt.Fprintln(stderr, "uso: sesim clock [pause | resume]")
		return 2
	}
	t, code := openGame(ctx, "clock", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	var err error
	switch action {
	case "pause":
		err = t.g.PauseClock()
	case "resume":
		err = t.g.ResumeClock()
	}
	if err != nil {
		return t.fail(err)
	}
	clock := t.g.Clock()
	if !clock.Paused {
		fmt.Fprintln(stdout, "O relógio da história está correndo.")
	}
	v, err := t.g.Story(ctx)
	if err != nil {
		return t.fail(err)
	}
	printDeadline(stdout, v.Deadline, clock)
	if clock.Paused {
		fmt.Fprintln(stdout, "A loja continua recebendo tráfego: só a história espera. Deploy, check, test e lab retomam o relógio.")
	}
	return 0
}

// runProfile: the player's career: level, experience, balance and statement.
func runProfile(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	t, code := openGame(ctx, "profile", args, stdout, stderr, true)
	if t == nil {
		return code
	}
	defer t.g.Close()
	p := t.g.Profile()
	fmt.Fprintf(stdout, "%s · %d de experiência", p.Level.Title, p.XP)
	if p.Next != nil {
		fmt.Fprintf(stdout, " (%s em %d)", p.Next.Title, p.Next.XP)
	}
	fmt.Fprintf(stdout, "\nSaldo: %s · salário por missão: %s\n", career.Money(p.Balance), career.Money(p.Level.Salary))
	fmt.Fprintf(stdout, "Próxima dica: %s · próximo experimento no laboratório: %s\n\nExtrato (mais recente primeiro):\n",
		priceLabel(t.g.NextHintPrice()), priceLabel(t.g.NextLabPrice()))
	for _, l := range p.Statement {
		fmt.Fprintf(stdout, "  %s  %-44s %12s\n", l.At.Local().Format("02/01 15:04"), l.Label, career.Money(l.Amount))
	}
	return 0
}

func priceLabel(price int) string {
	if price == 0 {
		return "grátis"
	}
	return career.Money(price)
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
	printDeadline(stdout, v.Deadline, t.g.Clock())
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
	if err := t.g.RecordBaseline(ctx, &sig); err != nil {
		return t.fail(err)
	}
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
	price := t.g.NextHintPrice()
	h, ok, err := t.g.Hint()
	if err != nil {
		return t.fail(err)
	}
	if !ok {
		fmt.Fprintln(stdout, "Não há mais dicas. As anteriores continuam valendo; releia com calma.")
		return 0
	}
	fmt.Fprintf(stdout, "Dica %d de %d · %s\n\n%s\n", h.Level, len(t.g.Pack().Hints), h.Kind, play.RenderBriefing(h.Text, play.CLIWords))
	if price > 0 {
		fmt.Fprintf(stdout, "\nEsta dica custou %s do seu salário (saldo: %s). `sesim profile` mostra o extrato.\n", career.Money(price), career.Money(t.g.Profile().Balance))
	}
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
	if err := t.g.MarkViewed("shell"); err != nil {
		return t.fail(err)
	}
	fmt.Fprintln(stdout, "Terminal na rede da loja: `psql` abre o banco (já autenticado); `exit` sai.")
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
