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
		{"válido", checkoutRequest{"ref-00000001", []store.Item{item(1, 2)}}, true},
		{"ref curta", checkoutRequest{"r1", []store.Item{item(1, 1)}}, false},
		{"sem itens", checkoutRequest{"ref-00000001", nil}, false},
		{"quantidade zero", checkoutRequest{"ref-00000001", []store.Item{item(1, 0)}}, false},
		{"unidades demais", checkoutRequest{"ref-00000001", []store.Item{item(1, 11)}}, false},
		{"produto repetido", checkoutRequest{"ref-00000001", []store.Item{item(1, 1), item(1, 2)}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateCheckout(tt.req) == ""; got != tt.wantOK {
				t.Errorf("validateCheckout ok = %v, esperado %v", got, tt.wantOK)
			}
		})
	}
}
