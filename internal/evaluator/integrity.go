package evaluator

import "context"

// Ledger is how the evaluator reconciles money and orders: the emulated
// gateway's record (outside the player's reach) against the orders the
// system stored. Only scenarios that grade integrity need it.
type Ledger interface {
	// Charges maps each order_ref starting with prefix to how many times the
	// gateway charged it.
	Charges(ctx context.Context, prefix string) (map[string]int, error)
	// PaidOrders maps each order_ref starting with prefix to how many paid
	// orders the database holds for it.
	PaidOrders(ctx context.Context, prefix string) (map[string]int, error)
}

// Integrity is one run's reconciliation.
type Integrity struct {
	Lost             int `json:"lost_orders"`       // charged, no paid order: the customer paid for nothing
	Phantom          int `json:"phantom_orders"`    // paid order, never charged
	DuplicateCharges int `json:"duplicate_charges"` // an order_ref charged more than once
	DuplicateOrders  int `json:"duplicate_orders"`  // an order_ref stored as more than one paid order
}

// Mismatches is every discrepancy; the objective wants zero.
func (i Integrity) Mismatches() int {
	return i.Lost + i.Phantom + i.DuplicateCharges + i.DuplicateOrders
}

// Reconcile compares the gateway's charges with the stored paid orders.
func Reconcile(charges, orders map[string]int) Integrity {
	var in Integrity
	for ref, n := range charges {
		if orders[ref] == 0 {
			in.Lost++
		}
		if n > 1 {
			in.DuplicateCharges++
		}
	}
	for ref, n := range orders {
		if charges[ref] == 0 {
			in.Phantom++
		}
		if n > 1 {
			in.DuplicateOrders++
		}
	}
	return in
}

// summed measures are judged on their total over the runs, not per run.
var summed = map[string]bool{"payment_mismatches": true}

// integrityMeasures are the measures a Ledger feeds.
var integrityMeasures = map[string]bool{
	"payment_mismatches": true, "lost_orders": true, "phantom_orders": true, "duplicate_charges": true,
}
