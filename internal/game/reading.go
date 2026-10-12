package game

import (
	"context"
	"fmt"
	"slices"

	"github.com/Bappoz/software-engineering-simulator/internal/scenario"
)

// ReadingView is one piece of documentation Slotthy recommends for the mission
// (ADR-0026), with her words rendered for the view asking.
type ReadingView struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Source  string `json:"source"`
	URL     string `json:"url"`
	Lang    string `json:"lang"`
	Minutes int    `json:"minutes"`
	Why     string `json:"why"`
}

// Reading is what Slotthy recommends reading for this scenario; words names the
// tools for the view asking. Empty when the scenario recommends nothing.
func (g *Game) Reading(ctx context.Context, words map[string]string) []ReadingView {
	list := g.Pack().Reading
	out := make([]ReadingView, 0, len(list))
	for _, r := range list {
		out = append(out, ReadingView{ID: r.ID, Title: r.Title, Source: r.Source, URL: r.URL, Lang: r.Lang, Minutes: r.Minutes,
			Why: g.render(ctx, r.Why, words)})
	}
	return out
}

// OpenedReading records that the player opened a recommended reading.
func (g *Game) OpenedReading(id string) error {
	if !slices.ContainsFunc(g.Pack().Reading, func(r scenario.Reading) bool { return r.ID == id }) {
		return fmt.Errorf("leitura desconhecida %q", id)
	}
	g.Event("reading_opened", map[string]any{"id": id, "paused": g.Clock().Paused})
	return nil
}
