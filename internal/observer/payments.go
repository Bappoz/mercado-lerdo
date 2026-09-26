// Package observer implements the observer-zone services. Payments emulates
// the external payment gateway and records every charge per order_ref, so the
// evaluator can detect lost or duplicated payments without trusting the app.
package observer

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

// Charge is one recorded payment attempt.
type Charge struct {
	OrderRef    string    `json:"order_ref"`
	AmountCents int       `json:"amount_cents"`
	At          time.Time `json:"at"`
}

// Payments is an in-memory payment gateway.
type Payments struct {
	mu      sync.Mutex
	charges map[string][]Charge
	total   int
	now     func() time.Time
	latency Latency
	sleep   func(time.Duration)
	keys    map[string]*keyed // Idempotency-Key -> its one charge
}

// keyed is the single outcome of an idempotency key; done closes when known.
type keyed struct {
	done   chan struct{}
	charge Charge
}

// NewPayments returns an empty gateway that answers after latency.
func NewPayments(latency Latency) *Payments {
	return &Payments{charges: map[string][]Charge{}, now: time.Now, latency: latency, sleep: time.Sleep, keys: map[string]*keyed{}}
}

// Handler routes the gateway API.
func (p *Payments) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /charge", p.charge)
	mux.HandleFunc("GET /charges/{ref}", p.chargesFor)
	mux.HandleFunc("GET /charges", p.list)
	mux.HandleFunc("GET /stats", p.stats)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func (p *Payments) charge(w http.ResponseWriter, r *http.Request) {
	var c Charge
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&c); err != nil ||
		c.OrderRef == "" || c.AmountCents <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order_ref and positive amount_cents required"})
		return
	}
	// An Idempotency-Key is charged once, like a real gateway: a repeat
	// (even one arriving while the first is in flight) gets the original.
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		p.mu.Lock()
		k, seen := p.keys[key]
		if !seen {
			k = &keyed{done: make(chan struct{})}
			p.keys[key] = k
		}
		p.mu.Unlock()
		if seen {
			<-k.done
			writeJSON(w, http.StatusOK, map[string]any{"charge": k.charge, "replayed": true})
			return
		}
		k.charge = p.capture(c)
		close(k.done)
		writeJSON(w, http.StatusCreated, k.charge)
		return
	}
	writeJSON(w, http.StatusCreated, p.capture(c))
}

// capture waits the gateway's latency and records the charge. The charge
// happens when the gateway answers: a client that gave up waiting has paid.
func (p *Payments) capture(c Charge) Charge {
	p.sleep(p.latency.For(c.OrderRef))
	c.At = p.now().UTC()
	p.mu.Lock()
	p.charges[c.OrderRef] = append(p.charges[c.OrderRef], c)
	p.total++
	p.mu.Unlock()
	return c
}

func (p *Payments) chargesFor(w http.ResponseWriter, r *http.Request) {
	ref := r.PathValue("ref")
	p.mu.Lock()
	charges := append([]Charge(nil), p.charges[ref]...)
	p.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"order_ref": ref, "count": len(charges), "charges": charges})
}

// RefCharges is how many times one order_ref was charged, and for how much in total.
type RefCharges struct {
	OrderRef    string    `json:"order_ref"`
	Count       int       `json:"count"`
	AmountCents int       `json:"amount_cents"`
	LastAt      time.Time `json:"last_at"`
}

// List returns the charges per order_ref starting with prefix, sorted by ref.
func (p *Payments) List(prefix string) []RefCharges {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := []RefCharges{}
	for ref, cs := range p.charges {
		if !strings.HasPrefix(ref, prefix) {
			continue
		}
		rc := RefCharges{OrderRef: ref, Count: len(cs)}
		for _, c := range cs {
			rc.AmountCents += c.AmountCents
			if c.At.After(rc.LastAt) {
				rc.LastAt = c.At
			}
		}
		out = append(out, rc)
	}
	slices.SortFunc(out, func(a, b RefCharges) int { return strings.Compare(a.OrderRef, b.OrderRef) })
	return out
}

func (p *Payments) list(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	if len(prefix) < 3 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prefix of at least 3 characters required"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"charges": p.List(prefix)})
}

// Stats summarizes the recorded charges.
type Stats struct {
	Charges    int `json:"charges"`
	OrderRefs  int `json:"order_refs"`
	Duplicated int `json:"duplicated_order_refs"`
}

func (p *Payments) stats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, p.Stats())
}

// Stats returns totals, counting order_refs charged more than once.
func (p *Payments) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := Stats{Charges: p.total, OrderRefs: len(p.charges)}
	for _, cs := range p.charges {
		if len(cs) > 1 {
			s.Duplicated++
		}
	}
	return s
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
