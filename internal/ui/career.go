package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/career"
	"github.com/Bappoz/software-engineering-simulator/internal/play"
)

const payroll = "Otávio (financeiro)"

// careerMails are Otávio's mails about the player's money in this scenario:
// the hiring on the first day of the campaign, and the payslip and any
// promotion when the mission passes (ADR-0023).
func careerMails(sess play.Session, p career.Profile, mission string) []Message {
	var out []Message
	if len(sess.History) == 0 && !sess.StartedAt.IsZero() {
		out = append(out, Message{ID: "career-hired", From: payroll, Subject: "Bem-vinda(o) à folha", At: sess.StartedAt.Add(time.Second),
			Markdown: fmt.Sprintf(`O seu **bônus de contratação** de %s já caiu na conta. O salário cai a cada missão concluída, com bônus quando dá certo: emergência resolvida no prazo, pedido atendido, poucas dicas, economia no orçamento.

**O que o salário paga** (é seu, não da empresa):

- dicas da Tabby: a primeira de cada missão é grátis, as seguintes custam mais a cada uma;
- horas extras no Laboratório: %d experimentos por missão são por conta da casa.

Ficou sem saldo? Eu adianto o salário. Ninguém fica sem dica por falta de dinheiro. O app **Perfil** mostra o seu nível e o extrato.`,
				career.Money(career.SigningBonus), career.FreeLabRuns)})
	}
	if sess.Phase != play.Completed {
		return out
	}
	for _, slip := range p.Payslips {
		if slip.Mission != mission {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Holerite de **%s**, pago como %s.\n\n| | |\n|---|---:|\n", slip.Title, slip.Level)
		for _, l := range slip.Lines {
			fmt.Fprintf(&b, "| %s | %s |\n", l.Label, cell(l.Amount))
		}
		fmt.Fprintf(&b, "| **Total** | **%s** |\n\n+%d de experiência. Saldo agora: %s.", cell(slip.Total), slip.XP, career.Money(p.Balance))
		out = append(out, Message{ID: "career-payslip-" + mission, From: payroll, Subject: "Holerite · " + slip.Title, At: sess.DoneAt.Add(time.Second), Markdown: b.String()})
	}
	for _, pr := range p.Promotions {
		if !pr.At.Equal(sess.DoneAt) {
			continue
		}
		out = append(out, Message{ID: "career-promotion-" + pr.To, From: payroll, Subject: "Promoção: " + pr.To, At: sess.DoneAt.Add(2 * time.Second),
			Markdown: fmt.Sprintf("O Álvaro pediu, eu fiz as contas, e desta vez elas fecham: você passa de **%s** para **%s**. A partir da próxima missão, o salário é **%s**.", pr.From, pr.To, career.Money(pr.Salary))})
	}
	return out
}

// cell keeps an amount on one line in a narrow table column.
func cell(v int) string { return strings.ReplaceAll(career.Money(v), " ", "\u00a0") }
