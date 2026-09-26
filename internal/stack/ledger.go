package stack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"github.com/Bappoz/software-engineering-simulator/internal/observer"
)

// Stack implements evaluator.Ledger.

// Charges asks the emulated gateway what it charged, per order_ref.
func (s *Stack) Charges(ctx context.Context, prefix string) (map[string]int, error) {
	list, err := s.ChargeList(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int, len(list))
	for _, c := range list {
		out[c.OrderRef] = c.Count
	}
	return out, nil
}

// ChargeList is the gateway's record for the order_refs starting with prefix.
func (s *Stack) ChargeList(ctx context.Context, prefix string) ([]observer.RefCharges, error) {
	a, err := s.Addr(ctx, "payments", netPlay)
	if err != nil {
		return nil, err
	}
	u := "http://" + netip.AddrPortFrom(a, 8090).String() + "/charges?prefix=" + url.QueryEscape(prefix)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("stack: charges: %w", err)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stack: charges: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stack: charges: gateway returned %s", resp.Status)
	}
	var body struct {
		Charges []observer.RefCharges `json:"charges"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("stack: charges: %w", err)
	}
	return body.Charges, nil
}

// PaidOrders counts the paid orders per order_ref in the live database, read
// as the superuser: the player's system is under test, not its reporting.
func (s *Stack) PaidOrders(ctx context.Context, prefix string) (map[string]int, error) {
	// The prefix comes from the evaluator (a run id), never from the player;
	// it is still passed as data, not spliced into SQL.
	// psql interpolates :'prefix' (quoted as a literal) in scripts read from
	// stdin, not in -c commands.
	res, err := s.d.Exec(ctx, s.Run, "postgres", 1, []string{
		"psql", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "nexora", "-At", "-F", "\t",
		"-v", "prefix=" + prefix + "%",
	}, strings.NewReader(`SELECT order_ref, count(*) FROM orders WHERE status = 'paid' AND order_ref LIKE :'prefix' GROUP BY order_ref;`))
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("stack: paid orders: psql exit %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	out := map[string]int{}
	for line := range strings.Lines(res.Stdout) {
		ref, n, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		c, err := strconv.Atoi(n)
		if err != nil {
			return nil, fmt.Errorf("stack: paid orders: %q: %w", line, err)
		}
		out[ref] = c
	}
	return out, nil
}
