package observer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDuplicateChargesAreCounted(t *testing.T) {
	p := NewPayments()
	srv := httptest.NewServer(p.Handler())
	defer srv.Close()

	post := func(body string) int {
		resp, err := http.Post(srv.URL+"/charge", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	for _, body := range []string{
		`{"order_ref":"ref-1","amount_cents":100}`,
		`{"order_ref":"ref-1","amount_cents":100}`,
		`{"order_ref":"ref-2","amount_cents":50}`,
	} {
		if code := post(body); code != http.StatusCreated {
			t.Fatalf("charge %s: status %d", body, code)
		}
	}
	for _, bad := range []string{`{}`, `{"order_ref":"x","amount_cents":0}`, `not json`} {
		if code := post(bad); code != http.StatusBadRequest {
			t.Errorf("charge %q: status %d, want 400", bad, code)
		}
	}

	if got, want := p.Stats(), (Stats{Charges: 3, OrderRefs: 2, Duplicated: 1}); got != want {
		t.Errorf("Stats() = %+v, want %+v", got, want)
	}

	resp, err := http.Get(srv.URL + "/charges/ref-1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct{ Count int }
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Count != 2 {
		t.Errorf("GET /charges/ref-1 count = %d (err %v), want 2", body.Count, err)
	}
}
