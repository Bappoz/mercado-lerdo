package seed

import (
	"slices"
	"testing"
)

func small(seed uint64) Config { return Config{Seed: seed, Users: 50, Products: 500, Orders: 200} }

func TestDeterministic(t *testing.T) {
	a, b := small(1), small(1)
	if !slices.EqualFunc(slices.Collect(a.ProductRows()), slices.Collect(b.ProductRows()), func(x, y Product) bool { return x == y }) {
		t.Fatal("same seed produced different products")
	}
	if !slices.Equal(slices.Collect(a.UserRows()), slices.Collect(b.UserRows())) {
		t.Fatal("same seed produced different users")
	}
	pa, pb := prices(a), prices(b)
	oa, ob := slices.Collect(a.OrderRows(pa)), slices.Collect(b.OrderRows(pb))
	if len(oa) != len(ob) {
		t.Fatal("order counts differ")
	}
	for i := range oa {
		if oa[i].UserID != ob[i].UserID || oa[i].TotalCents != ob[i].TotalCents || !slices.Equal(oa[i].Items, ob[i].Items) {
			t.Fatalf("order %d differs", i+1)
		}
	}
}

func TestDifferentSeedsDiffer(t *testing.T) {
	a := slices.Collect(small(1).ProductRows())
	b := slices.Collect(small(2).ProductRows())
	if a[0].Name == b[0].Name && a[1].Name == b[1].Name && a[2].Name == b[2].Name {
		t.Fatal("different seeds produced the same first products")
	}
}

func TestTablesAreIndependent(t *testing.T) {
	a := small(7)
	b := a
	b.Users = 99 // resizing users must not change products
	if slices.Collect(a.ProductRows())[42] != slices.Collect(b.ProductRows())[42] {
		t.Fatal("changing Users changed Products")
	}
}

func TestInvariants(t *testing.T) {
	c := small(3)
	skus := map[string]bool{}
	for p := range c.ProductRows() {
		if p.PriceCents <= 0 || p.Stock < 0 || p.CreatedAt.After(Base) || skus[p.SKU] {
			t.Fatalf("invalid product %+v", p)
		}
		skus[p.SKU] = true
	}
	emails := map[string]bool{}
	for u := range c.UserRows() {
		if emails[u.Email] {
			t.Fatalf("duplicate email %s", u.Email)
		}
		emails[u.Email] = true
	}
	pr := prices(c)
	for o := range c.OrderRows(pr) {
		if o.UserID < 1 || o.UserID > int64(c.Users) || len(o.Items) == 0 {
			t.Fatalf("invalid order %+v", o)
		}
		sum, seen := 0, map[int64]bool{}
		for _, it := range o.Items {
			if seen[it.ProductID] || it.PriceCents != pr[it.ProductID-1] {
				t.Fatalf("invalid item %+v in order %d", it, o.ID)
			}
			seen[it.ProductID] = true
			sum += it.PriceCents * it.Quantity
		}
		if sum != o.TotalCents {
			t.Fatalf("order %d total %d != items sum %d", o.ID, o.TotalCents, sum)
		}
	}
}

func TestValidate(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []Config{{Users: 0, Products: 1}, {Users: 1, Products: 0}, {Users: 1, Products: 1, Orders: -1}, {Users: 100_000, Products: 1}} {
		if c.Validate() == nil {
			t.Errorf("Validate(%+v) = nil, want error", c)
		}
	}
}

func prices(c Config) []int {
	var out []int
	for p := range c.ProductRows() {
		out = append(out, p.PriceCents)
	}
	return out
}
