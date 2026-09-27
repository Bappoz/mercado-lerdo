// Package career is the player's career at Mercado Lerdo: levels, experience,
// salary, bonuses and what the salary buys (docs/product/universe.md §4,
// ADR-0023). It is pure: the game hands it the finished missions and the
// purchases, and it derives the profile. Values are game values, to be
// calibrated in playtests; money is in whole reais.
package career

import (
	"fmt"
	"slices"
	"time"
)

// Level is one rung of the ladder.
type Level struct {
	Title  string `json:"title"`
	XP     int    `json:"xp"`     // experience needed to reach it
	Salary int    `json:"salary"` // per mission (about a month of the company)
}

// Ladder is Júnior I to Sênior III. About two missions per level: the six
// missions of Stage 1 take a player from Júnior I to Pleno I.
var Ladder = []Level{
	{"Júnior I", 0, 4000},
	{"Júnior II", 250, 4800},
	{"Júnior III", 550, 5800},
	{"Pleno I", 900, 7500},
	{"Pleno II", 1300, 9000},
	{"Pleno III", 1750, 10800},
	{"Sênior I", 2250, 13500},
	{"Sênior II", 2800, 16000},
	{"Sênior III", 3400, 19000},
}

// SigningBonus arrives on the first day, so the first missions can afford a hint.
const SigningBonus = 1500

// Mission is one finished mission, as the career sees it.
type Mission struct {
	ID                string
	Title             string
	DoneAt            time.Time
	Hints             int
	EmergenciesMet    int
	EmergenciesMissed int
	RequestsMet       int
	Cost, Budget      float64 // the modeled monthly cost at the PASS and its limit (0: unknown)
}

// Purchase is something the player bought with their salary.
type Purchase struct {
	At    time.Time `json:"at"`
	Item  string    `json:"item"`
	Price int       `json:"price"`
}

// Line is one line of a payslip or of the statement.
type Line struct {
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"` // hiring | salary | bonus | purchase
	Label  string    `json:"label"`
	Amount int       `json:"amount"` // negative for purchases
}

// Payslip is what one mission paid.
type Payslip struct {
	Mission string `json:"mission"`
	Title   string `json:"title"`
	Level   string `json:"level"` // the level it was paid at
	Lines   []Line `json:"lines"`
	Total   int    `json:"total"`
	XP      int    `json:"xp"`
}

// Promotion is a new level, reached at the end of a mission.
type Promotion struct {
	At     time.Time `json:"at"`
	From   string    `json:"from"`
	To     string    `json:"to"`
	Salary int       `json:"salary"`
}

// Profile is the player's career so far.
type Profile struct {
	Level      Level       `json:"level"`
	Next       *Level      `json:"next,omitempty"` // nil at the top
	XP         int         `json:"xp"`
	Balance    int         `json:"balance"`
	Statement  []Line      `json:"statement"` // newest first
	Payslips   []Payslip   `json:"payslips"`
	Promotions []Promotion `json:"promotions"`
}

// LevelAt is the highest level xp reaches.
func LevelAt(xp int) (Level, *Level) {
	i := 0
	for i+1 < len(Ladder) && xp >= Ladder[i+1].XP {
		i++
	}
	if i+1 < len(Ladder) {
		return Ladder[i], &Ladder[i+1]
	}
	return Ladder[i], nil
}

// pay is what a mission earns at a salary: the payslip lines and the XP.
func pay(m Mission, salary int) ([]Line, int) {
	lines := []Line{{At: m.DoneAt, Kind: "salary", Label: "Salário", Amount: salary}}
	xp := 150
	bonus := func(label string, percent, points int) {
		lines = append(lines, Line{At: m.DoneAt, Kind: "bonus", Label: label, Amount: salary * percent / 100})
		xp += points
	}
	// One line per kind of result, however many: a payslip never repeats a line.
	if n := m.EmergenciesMet; n > 0 {
		bonus(plural(n, "Bônus: emergência resolvida no prazo", "Bônus: %d emergências resolvidas no prazo"), 20*n, 60*n)
	}
	if n := m.RequestsMet; n > 0 {
		bonus(plural(n, "Bônus: pedido atendido", "Bônus: %d pedidos atendidos"), 5*n, 20*n)
	}
	switch {
	case m.Hints == 0:
		bonus("Bônus: sem nenhuma dica", 15, 40)
	case m.Hints <= 2:
		bonus("Bônus: poucas dicas", 5, 15)
	}
	if m.Budget > 0 && m.Cost <= 0.9*m.Budget {
		bonus("Bônus: economia (≤ 90 % do orçamento)", 10, 20)
	}
	return lines, xp
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return fmt.Sprintf(many, n)
}

// Compute derives the profile from the day the player was hired, the missions
// they finished (in order) and every purchase.
func Compute(hired time.Time, missions []Mission, purchases []Purchase) Profile {
	p := Profile{Statement: []Line{}, Payslips: []Payslip{}, Promotions: []Promotion{}}
	p.Statement = append(p.Statement, Line{At: hired, Kind: "hiring", Label: "Bônus de contratação", Amount: SigningBonus})
	for _, m := range missions {
		level, _ := LevelAt(p.XP)
		lines, xp := pay(m, level.Salary)
		slip := Payslip{Mission: m.ID, Title: m.Title, Level: level.Title, Lines: lines, XP: xp}
		for _, l := range lines {
			slip.Total += l.Amount
		}
		p.Payslips = append(p.Payslips, slip)
		p.Statement = append(p.Statement, lines...)
		p.XP += xp
		if now, _ := LevelAt(p.XP); now.Title != level.Title {
			p.Promotions = append(p.Promotions, Promotion{At: m.DoneAt, From: level.Title, To: now.Title, Salary: now.Salary})
		}
	}
	for _, b := range purchases {
		p.Statement = append(p.Statement, Line{At: b.At, Kind: "purchase", Label: b.Item, Amount: -b.Price})
	}
	for _, l := range p.Statement {
		p.Balance += l.Amount
	}
	// Newest first; on the same instant, earnings before spending.
	slices.SortStableFunc(p.Statement, func(a, b Line) int { return b.At.Compare(a.At) })
	p.Level, p.Next = LevelAt(p.XP)
	return p
}

// HintPrice is what the n-th hint of a mission costs (1-based): the first is
// free, the next ones cost more and more. A player with no money still gets
// it: the salary is advanced (a negative balance), never the hint refused.
func HintPrice(n int) int {
	if n <= 1 {
		return 0
	}
	return 50 + 100*(n-1)
}

// FreeLabRuns is how many load-lab experiments a mission includes.
const FreeLabRuns = 3

// LabPrice is what the n-th lab experiment of a mission costs (1-based).
func LabPrice(n int) int {
	if n <= FreeLabRuns {
		return 0
	}
	return 100
}

// Money formats reais the Brazilian way: R$ 1.500, -R$ 150.
func Money(v int) string {
	sign := ""
	if v < 0 {
		sign, v = "-", -v
	}
	s := fmt.Sprint(v)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return sign + "R$ " + s
}
