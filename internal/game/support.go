package game

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"
)

// Ticket is a customer complaint. Tickets come from what the story's clients
// actually experienced (the load generator's view and the payment gateway's
// record), never from a script: a healthy system has none.
type Ticket struct {
	ID       string    `json:"id"`
	Kind     string    `json:"kind"` // error | slow | lost_payment | double_charge
	Endpoint string    `json:"endpoint,omitempty"`
	Count    int       `json:"count"` // affected requests or orders
	Subject  string    `json:"subject"`
	Quote    string    `json:"quote"`
	Since    time.Time `json:"since"` // start of the window the ticket counts
}

const (
	supportWindow = 10 * time.Minute
	// inFlight is how old a charge must be before its order counts as lost:
	// younger ones may still be committing.
	inFlight = 30 * time.Second
	// slowAfter is when a page is slow enough for customers to write in.
	slowAfter = 1.5 // seconds, p95 over five minutes
)

var pageName = map[string]string{
	"search": "a busca", "products": "a lista de produtos", "product": "a página de produto", "checkout": "o checkout",
}

// inPage is "em" + the page, contracted as Portuguese does ("no checkout").
var inPage = map[string]string{
	"search": "na busca", "products": "na lista de produtos", "product": "na página de produto", "checkout": "no checkout",
}

// Tickets lists the complaints of the last ten minutes, most affected first.
func (g *Game) Tickets(ctx context.Context) ([]Ticket, error) {
	st, err := g.requireStack()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	since := now.Add(-supportWindow)
	sel := fmt.Sprintf(`testid=~"%s.*",name!="setup_login",name!=""`, backgroundPrefix)
	var out []Ticket

	errs, err := st.QueryVector(ctx, fmt.Sprintf(`sum by (name) (increase(k6_http_reqs_total{%s,expected_response="false"}[10m]))`, sel), now)
	if err != nil {
		return nil, err
	}
	for _, s := range errs {
		n := int(s.Value + 0.5)
		if n < 3 { // one-off failures do not make people write in
			continue
		}
		ep := s.Labels["name"]
		t := Ticket{ID: "error-" + ep, Kind: "error", Endpoint: ep, Count: n, Since: since,
			Subject: "Erro " + inPage[ep]}
		if ep == "checkout" {
			t.Quote = "Tentei finalizar a compra e deu erro. Tentei de novo e nada."
		} else {
			t.Quote = "A página deu erro na minha cara. Isso aqui está funcionando?"
		}
		out = append(out, t)
	}

	slow, err := st.QueryVector(ctx, fmt.Sprintf(`histogram_quantile(0.95, sum by (name) (rate(k6_http_req_duration_seconds{%s,name!="checkout"}[5m])))`, sel), now)
	if err != nil {
		return nil, err
	}
	for _, s := range slow {
		if s.Value < slowAfter || s.Value != s.Value { // NaN: no requests
			continue
		}
		ep := s.Labels["name"]
		out = append(out, Ticket{ID: "slow-" + ep, Kind: "slow", Endpoint: ep, Count: 1, Since: now.Add(-5 * time.Minute),
			Subject: fmt.Sprintf("%s está muito lenta", capitalize(pageName[ep])),
			Quote:   fmt.Sprintf("Demora uma eternidade para carregar (%.1f s para 5 %% dos clientes). Desisti.", s.Value)})
	}

	// Money: the gateway's record against the orders stored since the
	// current traffic started (a Production Test resets the database).
	if bg := g.Session().Background; bg != "" {
		charges, err := st.ChargeList(ctx, bg+"-")
		if err != nil {
			return nil, err
		}
		orders, err := st.PaidOrders(ctx, bg+"-")
		if err != nil {
			return nil, err
		}
		lost, dup := 0, 0
		for _, c := range charges {
			if c.Count > 1 {
				dup++
			}
			if orders[c.OrderRef] == 0 && now.Sub(c.LastAt) > inFlight {
				lost++
			}
		}
		if lost > 0 {
			out = append(out, Ticket{ID: "lost-payment", Kind: "lost_payment", Count: lost, Since: since,
				Subject: "Fui cobrado e o pedido não existe",
				Quote:   "O valor saiu do meu cartão, mas o pedido não aparece em lugar nenhum. Quero meu dinheiro de volta."})
		}
		if dup > 0 {
			out = append(out, Ticket{ID: "double-charge", Kind: "double_charge", Count: dup, Since: since,
				Subject: "Cobrança em dobro",
				Quote:   "Fiz UMA compra e vieram duas cobranças na fatura."})
		}
	}
	// Money first, then errors, then slowness; within a kind, most affected first.
	rank := map[string]int{"lost_payment": 0, "double_charge": 1, "error": 2, "slow": 3}
	slices.SortFunc(out, func(a, b Ticket) int {
		return cmp.Or(cmp.Compare(rank[a.Kind], rank[b.Kind]), cmp.Compare(b.Count, a.Count), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-'a'+'A') + s[1:]
}
