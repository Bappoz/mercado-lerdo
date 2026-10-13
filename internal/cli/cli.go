// Package cli implements the sesim commands. It holds no game logic: it only
// parses arguments and delegates to the other packages.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Bappoz/software-engineering-simulator/internal/buildinfo"
	"github.com/Bappoz/software-engineering-simulator/internal/workload"
)

const usage = `sesim — Mercado Lerdo, o simulador de engenharia de software

Uso:
  sesim <comando> [opções]

Para jogar:
  play           começa (ou retoma) o cenário: workspace, loja, briefing e tráfego
  next           encerra a fase calma; o incidente começa
  status         os sinais da produção agora (tráfego, erros, latência, CPU)
  check          Quick Check: aplica o seu workspace e mede ~45 s (indicativo)
  test           Production Test (o que vale); numa partida, mostra o debrief no PASS
  hint           a próxima dica (5 níveis)
  slotthy          o que a Slotthy diz no treinamento (ok | ajuda | responder <letra> | passo [n])
  reading        a documentação que a Slotthy recomenda para a missão
  clock          o relógio da história (pause | resume): pare para ler sem perder prazo
  shell          terminal dentro da rede da loja
  deploy         coloca o workspace em produção (código, migrações, topology.yaml, nginx)
  topology       a topologia do workspace, o custo modelado e as mudanças pendentes
  support        reclamações dos clientes nos últimos 10 minutos
  inbox          mensagens do pessoal (pedidos, broncas, emergências) e o prazo aberto
  profile        a sua carreira: nível, experiência, saldo do salário e extrato
  lab            experimento de carga contra a produção, sem nota (--profile --rate --duration --crash)
  campaign       as missões do stage e o que você já concluiu
  advance        depois de um PASS, segue para o próximo cenário (--canonical: a partir do estado oficial)
  docs [c]       o catálogo de componentes, ou a página de um componente
  install <c>    acrescenta um componente opcional ao topology.yaml (o deploy aplica)
  stop           para o tráfego de fundo (--down: também desliga a loja)
  ui             LerdOS: o mesmo jogo no navegador (só em 127.0.0.1)

Ferramentas:
  sandbox          up | ls | exec | down <run> | gc [--all]: sandboxes pela API do Docker
  scenario verify  roda o início quebrado e cada solução de referência; aponta vereditos inesperados
  load-summary     lê um resumo do k6 (JSON) na entrada padrão e mostra números por endpoint
  dev              comandos de desenvolvimento da stack local
  version          mostra a versão do binário
  help             mostra esta ajuda
`

// Run executes the command in args (without the program name) and returns the exit code.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, buildinfo.Read())
		return 0
	case "load-summary":
		res, err := workload.ParseK6Summary(stdin)
		if err != nil {
			fmt.Fprintln(stderr, "sesim:", err)
			return 1
		}
		res.WriteTable(stdout)
		return 0
	case "test":
		return runTest(ctx, args[1:], stdout, stderr)
	case "play":
		return runPlay(ctx, args[1:], stdout, stderr)
	case "next":
		return runNext(ctx, args[1:], stdout, stderr)
	case "status":
		return runStatus(ctx, args[1:], stdout, stderr)
	case "check":
		return runCheck(ctx, args[1:], stdout, stderr)
	case "hint":
		return runHint(ctx, args[1:], stdout, stderr)
	case "shell":
		return runShell(ctx, args[1:], stdout, stderr)
	case "stop":
		return runStop(ctx, args[1:], stdout, stderr)
	case "ui":
		return runUI(ctx, args[1:], stdout, stderr)
	case "deploy":
		return runDeploy(ctx, args[1:], stdout, stderr)
	case "topology":
		return runTopology(ctx, args[1:], stdout, stderr)
	case "profile":
		return runProfile(ctx, args[1:], stdout, stderr)
	case "inbox":
		return runInbox(ctx, args[1:], stdout, stderr)
	case "slotthy":
		return runSlotthy(ctx, args[1:], stdout, stderr)
	case "reading":
		return runReading(ctx, args[1:], stdout, stderr)
	case "clock":
		return runClock(ctx, args[1:], stdout, stderr)
	case "support":
		return runSupport(ctx, args[1:], stdout, stderr)
	case "lab":
		return runLab(ctx, args[1:], stdout, stderr)
	case "campaign":
		return runCampaign(ctx, args[1:], stdout, stderr)
	case "advance":
		return runAdvance(ctx, args[1:], stdout, stderr)
	case "docs":
		return runDocs(ctx, args[1:], stdout, stderr)
	case "install":
		return runInstall(ctx, args[1:], stdout, stderr)
	case "scenario":
		if len(args) < 2 || args[1] != "verify" {
			fmt.Fprintln(stderr, "uso: sesim scenario verify [opções]")
			return 2
		}
		return runScenarioVerify(ctx, args[2:], stdout, stderr)
	case "dev":
		return runDev(ctx, args[1:], stdout, stderr)
	case "sandbox":
		return runSandbox(ctx, args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "sesim: comando desconhecido %q\n\n%s", args[0], usage)
		return 2
	}
}

// ptFlags makes fs report parse errors and print its options in PT-BR: the
// flag package writes its own messages in English.
func ptFlags(fs *flag.FlagSet, stderr io.Writer) {
	fs.SetOutput(flagErrors{stderr})
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Opções de sesim %s:\n", fs.Name())
		fs.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(stderr, "  -%-12s %s", f.Name, f.Usage)
			if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
				fmt.Fprintf(stderr, " (padrão: %s)", f.DefValue)
			}
			fmt.Fprintln(stderr)
		})
	}
}

// flagErrors translates the flag package's parse errors on their way out.
type flagErrors struct{ w io.Writer }

var flagWords = strings.NewReplacer(
	"flag provided but not defined: ", "opção desconhecida: ",
	"flag needs an argument: ", "falta o valor da opção ",
	"invalid boolean value ", "valor inválido ",
	"invalid boolean flag ", "opção inválida ",
	"invalid value ", "valor inválido ",
	" for flag ", " para a opção ",
	" for -", " para -",
	"parse error", "não é um valor válido",
	"value out of range", "valor fora do intervalo",
)

func (e flagErrors) Write(p []byte) (int, error) {
	if _, err := flagWords.WriteString(e.w, string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}
