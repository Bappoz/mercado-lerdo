package career

import (
	"testing"
	"time"
)

func TestLadderIsOrdered(t *testing.T) {
	if len(Ladder) != 9 || Ladder[0].XP != 0 {
		t.Fatalf("ladder = %+v", Ladder)
	}
	for i := 1; i < len(Ladder); i++ {
		if Ladder[i].XP <= Ladder[i-1].XP || Ladder[i].Salary <= Ladder[i-1].Salary {
			t.Errorf("%s does not climb from %s", Ladder[i].Title, Ladder[i-1].Title)
		}
	}
	if l, next := LevelAt(0); l.Title != "Júnior I" || next == nil || next.Title != "Júnior II" {
		t.Errorf("start = %v, %v", l, next)
	}
	if l, next := LevelAt(99999); l.Title != "Sênior III" || next != nil {
		t.Errorf("top = %v, %v", l, next)
	}
}

func TestCompute(t *testing.T) {
	hired := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	at := func(h int) time.Time { return hired.Add(time.Duration(h) * time.Hour) }
	perfect := Mission{ID: "c1", Title: "Busca Lenta", DoneAt: at(1), Hints: 0, EmergenciesMet: 1, RequestsMet: 1, Cost: 70, Budget: 100}
	p := Compute(hired, []Mission{perfect}, nil)
	// Júnior I: 4000 + 20 % + 5 % + 15 % + 10 % = 4000 + 800 + 200 + 600 + 400.
	if got := p.Payslips[0].Total; got != 6000 {
		t.Errorf("perfect mission paid %d, want 6000", got)
	}
	if p.Balance != SigningBonus+6000 {
		t.Errorf("balance = %d", p.Balance)
	}
	// XP 150 + 60 + 20 + 40 + 20 = 290: a promotion to Júnior II at the end of the mission.
	if p.XP != 290 || p.Level.Title != "Júnior II" || len(p.Promotions) != 1 || p.Promotions[0].Salary != 4800 {
		t.Errorf("xp %d, level %s, promotions %+v", p.XP, p.Level.Title, p.Promotions)
	}

	// The next mission is paid at the new level; a rough one earns only the salary.
	rough := Mission{ID: "c2", DoneAt: at(2), Hints: 5, EmergenciesMissed: 1, Cost: 110, Budget: 110}
	p = Compute(hired, []Mission{perfect, rough}, []Purchase{{At: at(2), Item: "Dica 2", Price: 150}})
	if slip := p.Payslips[1]; slip.Level != "Júnior II" || slip.Total != 4800 || slip.XP != 150 {
		t.Errorf("rough payslip = %+v", slip)
	}
	if p.Balance != SigningBonus+6000+4800-150 {
		t.Errorf("balance with a purchase = %d", p.Balance)
	}
	if p.Statement[0].At.Before(p.Statement[len(p.Statement)-1].At) {
		t.Error("the statement must be newest first")
	}
}

func TestPayslipLinesAreUnique(t *testing.T) {
	p := Compute(time.Now(), []Mission{{ID: "m", DoneAt: time.Now(), RequestsMet: 2, Hints: 9}}, nil)
	seen := map[string]bool{}
	for _, l := range p.Payslips[0].Lines {
		if seen[l.Label] {
			t.Errorf("repeated line %q", l.Label)
		}
		seen[l.Label] = true
	}
	if !seen["Bônus: 2 pedidos atendidos"] || p.Payslips[0].Total != 4000+400 {
		t.Errorf("lines %v, total %d", seen, p.Payslips[0].Total)
	}
}

func TestPricesNeverBlock(t *testing.T) {
	if HintPrice(1) != 0 || HintPrice(2) != 150 || HintPrice(5) != 450 {
		t.Errorf("hint prices %d %d %d", HintPrice(1), HintPrice(2), HintPrice(5))
	}
	// Every hint of C1 is affordable from the signing bonus alone.
	total := 0
	for n := 1; n <= 5; n++ {
		total += HintPrice(n)
	}
	if total > SigningBonus {
		t.Errorf("all five hints cost %d, more than the signing bonus", total)
	}
	if LabPrice(FreeLabRuns) != 0 || LabPrice(FreeLabRuns+1) != 100 {
		t.Error("the lab's free runs")
	}
}

func TestMoney(t *testing.T) {
	for v, want := range map[int]string{0: "R$ 0", 150: "R$ 150", 1500: "R$ 1.500", 1234567: "R$ 1.234.567", -150: "-R$ 150"} {
		if got := Money(v); got != want {
			t.Errorf("Money(%d) = %q, want %q", v, got, want)
		}
	}
}
