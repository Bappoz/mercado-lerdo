package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/career"
	"github.com/Bappoz/software-engineering-simulator/internal/play"
)

func TestCareerMails(t *testing.T) {
	start := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	done := start.Add(time.Hour)
	s := play.Session{Phase: play.Calm, StartedAt: start}
	if m := careerMails(s, career.Compute(start, nil, nil), "c1"); len(m) != 1 || m[0].ID != "career-hired" || !strings.Contains(m[0].Markdown, "R$ 1.500") {
		t.Fatalf("first day: %+v", m)
	}
	s.Phase, s.DoneAt = play.Completed, done
	p := career.Compute(start, []career.Mission{{ID: "c1", Title: "Busca Lenta", DoneAt: done, EmergenciesMet: 1, Cost: 70, Budget: 100}}, nil)
	ids := map[string]bool{}
	for _, m := range careerMails(s, p, "c1") {
		ids[m.ID] = true
	}
	if !ids["career-payslip-c1"] || !ids["career-promotion-Júnior II"] {
		t.Errorf("mails at the PASS: %v", ids)
	}
	// Later in the campaign there is no second welcome.
	s.History = []play.Finished{{ID: "c0"}}
	for _, m := range careerMails(s, p, "c1") {
		if m.ID == "career-hired" {
			t.Error("hired twice")
		}
	}
}
