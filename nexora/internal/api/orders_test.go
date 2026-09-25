package api

import (
	"testing"

	"nexora/internal/store"
)

func TestValidateCheckout(t *testing.T) {
	item := func(id int64, q int) store.Item { return store.Item{ProductID: id, Quantity: q} }
	tests := []struct {
		name   string
		req    checkoutRequest
		wantOK bool
	}{
		{"valid", checkoutRequest{"ref-00000001", []store.Item{item(1, 2)}}, true},
		{"short ref", checkoutRequest{"r1", []store.Item{item(1, 1)}}, false},
		{"no items", checkoutRequest{"ref-00000001", nil}, false},
		{"zero quantity", checkoutRequest{"ref-00000001", []store.Item{item(1, 0)}}, false},
		{"too many units", checkoutRequest{"ref-00000001", []store.Item{item(1, 11)}}, false},
		{"duplicate product", checkoutRequest{"ref-00000001", []store.Item{item(1, 1), item(1, 2)}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateCheckout(tt.req) == ""; got != tt.wantOK {
				t.Errorf("validateCheckout ok = %v, want %v", got, tt.wantOK)
			}
		})
	}
}
