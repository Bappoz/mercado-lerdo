package ui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Bappoz/software-engineering-simulator/internal/game"
	"github.com/Bappoz/software-engineering-simulator/internal/play"
)

// MaxSaves is how many games the save menu keeps (ADR-0025).
const MaxSaves = 3

// SaveSlots are the workspaces of the save slots: base, base-2, base-3.
func SaveSlots(base string) ([]string, error) {
	abs, err := filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	out := []string{abs}
	for i := 2; i <= MaxSaves; i++ {
		out = append(out, fmt.Sprintf("%s-%d", abs, i))
	}
	return out, nil
}

// SaveView is one slot of the save menu.
type SaveView struct {
	Slot    int  `json:"slot"` // 1-based
	Current bool `json:"current"`
	game.SaveSummary
}

var errSavesFull = errors.New("os três saves estão ocupados: apague um para começar outro")

func samePath(a, b string) bool {
	x, err1 := filepath.Abs(a)
	y, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && filepath.Clean(x) == filepath.Clean(y)
}

// saveViews reads every slot; a slot that cannot be read shows as taken.
func (s *Server) saveViews() []SaveView {
	cur := s.gm().Session().Workspace
	if s.saves == nil {
		return []SaveView{{Slot: 1, Current: true}}
	}
	views := make([]SaveView, 0, len(s.saves))
	for i, ws := range s.saves {
		sum, err := game.Summarize(s.root, ws)
		if err != nil {
			s.log.Warn("save unreadable", "workspace", ws, "err", err)
			sum = game.SaveSummary{Scenario: "save ilegível"}
		}
		views = append(views, SaveView{Slot: i + 1, Current: samePath(ws, cur), SaveSummary: sum})
	}
	return views
}

func (s *Server) listSaves(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"saves": s.saveViews(), "max": MaxSaves})
}

// slotOf reads {"slot": n} and checks it names a slot.
func (s *Server) slotOf(w http.ResponseWriter, r *http.Request) (int, bool) {
	var body struct {
		Slot int `json:"slot"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return 0, false
	}
	if s.saves == nil || body.Slot < 1 || body.Slot > len(s.saves) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("save %d não existe", body.Slot))
		return 0, false
	}
	if s.busy() {
		writeError(w, http.StatusConflict, errBusy)
		return 0, false
	}
	return body.Slot - 1, true
}

func (s *Server) selectSave(w http.ResponseWriter, r *http.Request) {
	i, ok := s.slotOf(w, r)
	if !ok {
		return
	}
	if err := s.switchTo(r.Context(), i); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.listSaves(w, r)
}

// newSave switches to the first empty slot: the game starts from the menu's
// intro, then the inbox.
func (s *Server) newSave(w http.ResponseWriter, r *http.Request) {
	if s.saves == nil {
		writeError(w, http.StatusConflict, errors.New("este LerdOS abre um workspace só"))
		return
	}
	if s.busy() {
		writeError(w, http.StatusConflict, errBusy)
		return
	}
	for i, v := range s.saveViews() {
		if v.Empty {
			if err := s.switchTo(r.Context(), i); err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			s.listSaves(w, r)
			return
		}
	}
	writeError(w, http.StatusConflict, errSavesFull)
}

// deleteSave turns the slot's store off and removes its workspace: the code
// and the progress of that game are gone.
func (s *Server) deleteSave(w http.ResponseWriter, r *http.Request) {
	i, ok := s.slotOf(w, r)
	if !ok {
		return
	}
	ws := s.saves[i]
	if _, err := os.Stat(ws); errors.Is(err, os.ErrNotExist) {
		s.listSaves(w, r)
		return
	}
	// Never remove a directory the game did not create.
	if _, err := os.Stat(play.StateDir(ws)); err != nil {
		writeError(w, http.StatusConflict, fmt.Errorf("%s não parece um save do jogo; não vou apagar", ws))
		return
	}
	ctx := r.Context()
	current := samePath(ws, s.gm().Session().Workspace)
	g := s.gm()
	if !current {
		var err error
		if g, err = s.open(ctx, ws); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		defer g.Close()
	}
	if g.Running() {
		if err := g.Stop(ctx, true); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("não consegui desligar a loja deste save: %w", err))
			return
		}
	}
	if err := os.RemoveAll(ws); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.log.Info("save deleted", "slot", i+1, "workspace", ws)
	if current {
		// The slot stays selected, now empty.
		ng, err := s.open(ctx, ws)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.swap(ng)
	}
	s.listSaves(w, r)
}

// switchTo makes slot i the save being played. Only one store runs at a
// time on the player's machine: the previous save's store is turned off (its
// code and progress stay).
func (s *Server) switchTo(ctx context.Context, i int) error {
	old := s.gm()
	if samePath(old.Session().Workspace, s.saves[i]) {
		return nil
	}
	ng, err := s.open(ctx, s.saves[i])
	if err != nil {
		return err
	}
	if old.Running() {
		if err := old.Stop(ctx, true); err != nil {
			ng.Close()
			return fmt.Errorf("não consegui desligar a loja do save atual: %w", err)
		}
	}
	s.swap(ng)
	if ng.Running() {
		s.watchStats(ng)
	}
	return nil
}

// watchStats feeds the Pulse's CPU for g on the server's lifetime, not the
// request's: the switch returns, the watcher stays.
func (s *Server) watchStats(g *game.Game) {
	if err := g.WatchStats(s.ctx); err != nil {
		s.log.Warn("container stats unavailable; the dashboard will miss CPU", "err", err)
	}
}

// swap installs g as the current game and closes the previous one; what the
// last job showed belonged to the previous save.
func (s *Server) swap(g *game.Game) {
	s.gmu.Lock()
	old := s.cur
	s.cur = g
	s.gmu.Unlock()
	if old != nil && old != g {
		old.Close()
	}
	s.mu.Lock()
	s.job, s.lastTickets = nil, nil
	s.mu.Unlock()
}
