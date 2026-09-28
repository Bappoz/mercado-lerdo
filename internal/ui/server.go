// Package ui serves LerdOS, the game's web UI, on the loopback interface:
// the embedded SPA, a JSON API over internal/game and a terminal over
// WebSocket. Security follows docs/security/threat-model.md T1–T5.
package ui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/game"
	"github.com/Bappoz/software-engineering-simulator/internal/play"
	"github.com/Bappoz/software-engineering-simulator/internal/topology"
)

// Words name the game's tools in the scenario's texts as LerdOS shows them
// (the terminal's are play.CLIWords).
var Words = map[string]string{
	"status":  "o **Pulse**",
	"shell":   "o **Terminal**",
	"check":   "o **Quick Check** (app Testes)",
	"test":    "o **Production Test** (app Testes)",
	"hint":    "a **Tabby** (pedir dica)",
	"next":    "clique em **Começar o expediente**, aqui embaixo",
	"deploy":  "o **Deploy**",
	"support": "o **Suporte**",
	"lab":     "o **Laboratório de carga**",
	// Where the player reads the topology and the catalog.
	"architecture": "o app **Arquitetura**",
	"docs":         "o app **Docs**",
}

// Limits on what the browser may send.
const (
	maxJSONBody  = 8 << 10
	maxNotesBody = 256 << 10
)

// Server is the UI of one game. It runs one long operation at a time.
type Server struct {
	g      *game.Game
	static fs.FS
	token  string
	prefs  string // path of the UI preferences file
	log    *slog.Logger
	ctx    context.Context // lifetime of the server; long jobs run on it

	hosts map[string]bool // allowed Host headers (T2), set by Listen

	mu          sync.Mutex
	job         *Job
	nextJob     int
	lastTickets []game.Ticket // shown while a job runs (tickets)
}

// New creates the server. ctx bounds every job it starts.
func New(ctx context.Context, g *game.Game, static fs.FS, prefsPath string, log *slog.Logger) (*Server, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("ui: token: %w", err)
	}
	return &Server{g: g, static: static, token: hex.EncodeToString(b), prefs: prefsPath, log: log, ctx: ctx}, nil
}

// Token authenticates the browser; it goes in the URL fragment only.
func (s *Server) Token() string { return s.token }

// Listen binds addr, which must be a loopback IPv4 address (T4).
func (s *Server) Listen(addr string) (net.Listener, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("ui: %w", err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() || ip.To4() == nil {
		return nil, fmt.Errorf("ui: refusing to listen on %s: only 127.0.0.0/8 is allowed", host)
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(s.ctx, "tcp4", addr)
	if err != nil {
		return nil, fmt.Errorf("ui: %w", err)
	}
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	s.hosts = map[string]bool{net.JoinHostPort(host, port): true, "localhost:" + port: true}
	return ln, nil
}

// Handler is the whole UI: static files, API and terminal.
func (s *Server) Handler() http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /api/state", s.state)
	api.HandleFunc("POST /api/play", s.startJob("play"))
	api.HandleFunc("POST /api/check", s.startJob("check"))
	api.HandleFunc("POST /api/test", s.startJob("test"))
	api.HandleFunc("POST /api/deploy", s.startJob("deploy"))
	api.HandleFunc("POST /api/lab", s.startJob("lab"))
	api.HandleFunc("POST /api/advance", s.startJob("advance"))
	api.HandleFunc("GET /api/campaign", s.campaign)
	api.HandleFunc("GET /api/profile", s.profile)
	api.HandleFunc("GET /api/git/status", s.gitStatus)
	api.HandleFunc("GET /api/git/diff", s.gitDiff)
	api.HandleFunc("GET /api/git/log", s.gitLog)
	api.HandleFunc("POST /api/git/commit", s.gitCommit)
	api.HandleFunc("GET /api/topology", s.topology)
	api.HandleFunc("GET /api/catalog", s.catalog)
	api.HandleFunc("POST /api/install", s.install)
	api.HandleFunc("POST /api/next", s.next)
	api.HandleFunc("POST /api/hint", s.hint)
	api.HandleFunc("POST /api/stop", s.stop)
	api.HandleFunc("GET /api/pulse", s.pulse)
	api.HandleFunc("GET /api/tickets", s.tickets)
	api.HandleFunc("POST /api/viewed", s.viewed)
	api.HandleFunc("GET /api/notes", s.getNotes)
	api.HandleFunc("PUT /api/notes", s.putNotes)
	api.HandleFunc("GET /api/prefs", s.getPrefs)
	api.HandleFunc("PUT /api/prefs", s.putPrefs)

	mux := http.NewServeMux()
	mux.Handle("/api/", s.requireToken(api))
	// The browser cannot set headers on a WebSocket: the terminal checks the
	// Origin and authenticates with its first message instead (T3).
	mux.HandleFunc("GET /ws/terminal", s.terminal)
	mux.HandleFunc("/", s.files)
	return s.guard(mux)
}

// guard rejects foreign Host headers (DNS rebinding, T2) and sets the
// security headers of every response (T5).
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hosts[r.Host] {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; "+
			"connect-src 'self' ws://"+r.Host+"; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// requireToken checks the bearer token of API calls (CSRF from other pages, T1).
func (s *Server) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || !s.validToken(got) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) validToken(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}

// files serves the SPA; unknown paths get index.html (client-side routes).
func (s *Server) files(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	if st, err := fs.Stat(s.static, name); err != nil || st.IsDir() {
		name = "index.html"
	}
	if _, err := fs.Stat(s.static, "index.html"); err != nil {
		http.Error(w, "LerdOS was not built into this binary: run `just build`.", http.StatusServiceUnavailable)
		return
	}
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // content-hashed names
	}
	http.ServeFileFS(w, r, s.static, name) //nolint:gosec // G703: an fs.FS rejects ".." and absolute names (fs.ValidPath); name is also path.Clean-ed
}

// ---- state ----

// Message is one mail of the inbox.
type Message struct {
	ID       string    `json:"id"`
	From     string    `json:"from"`
	Subject  string    `json:"subject"`
	At       time.Time `json:"at"`
	Markdown string    `json:"markdown"`
	Kind     string    `json:"kind,omitempty"` // a story event's kind (ADR-0022); empty for the scenario's own mails
}

// HintView is a revealed hint, rendered for the UI.
type HintView struct {
	Level int    `json:"level"`
	Kind  string `json:"kind"`
	Text  string `json:"text"`
}

type stateView struct {
	Scenario   map[string]string `json:"scenario"`
	Session    *play.Session     `json:"session"` // nil before the first start
	Running    bool              `json:"running"`
	Messages   []Message         `json:"messages"`
	Hints      []HintView        `json:"hints"`
	HintsTotal int               `json:"hints_total"`
	Job        *Job              `json:"job"`
	LastTest   any               `json:"last_test"`
	Deadline   *game.Deadline    `json:"deadline"` // the most urgent open request of the story
	Career     *careerView       `json:"career"`   // nil before the first game (ADR-0023)
}

// careerView is the career at a glance: the top bar, Tabby's hint price.
type careerView struct {
	Level     string `json:"level"`
	Next      string `json:"next,omitempty"`
	XP        int    `json:"xp"`
	LevelXP   int    `json:"level_xp"` // where the current level starts
	NextXP    int    `json:"next_xp,omitempty"`
	Balance   int    `json:"balance"`
	HintPrice int    `json:"hint_price"` // the next hint of this mission
	LabPrice  int    `json:"lab_price"`  // the next lab run of this mission
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	md := s.g.Pack().Manifest
	v := stateView{
		Scenario:   map[string]string{"id": md.Metadata.ID, "title": md.Metadata.Title, "version": md.Metadata.Version},
		Running:    s.g.Running(),
		Messages:   []Message{},
		Hints:      []HintView{},
		HintsTotal: len(s.g.Pack().Hints),
		Job:        s.currentJob(),
	}
	if rep := s.g.LastTest(); rep != nil {
		v.LastTest = rep
	}
	if s.g.HasSession() {
		sess := s.g.Session()
		v.Session = &sess
		msgs, err := s.messages(r.Context(), sess)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		// The story's events land in the same inbox, in the order they happened.
		story, err := s.g.Story(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, m := range story.Messages {
			msgs = append(msgs, Message{ID: m.ID, From: m.From, Subject: m.Title, At: m.At, Kind: m.Kind,
				Markdown: play.RenderBriefing(m.Text, Words)})
		}
		prof := s.g.Profile()
		msgs = append(msgs, careerMails(sess, prof, s.g.Pack().Manifest.Metadata.ID)...)
		slices.SortStableFunc(msgs, func(a, b Message) int { return a.At.Compare(b.At) })
		v.Messages = msgs
		v.Deadline = story.Deadline
		cv := &careerView{Level: prof.Level.Title, XP: prof.XP, LevelXP: prof.Level.XP, Balance: prof.Balance,
			HintPrice: s.g.NextHintPrice(), LabPrice: s.g.NextLabPrice()}
		if prof.Next != nil {
			cv.Next, cv.NextXP = prof.Next.Title, prof.Next.XP
		}
		v.Career = cv
		for _, h := range s.g.Hints() {
			v.Hints = append(v.Hints, HintView{Level: h.Level, Kind: h.Kind, Text: play.RenderBriefing(h.Text, Words)})
		}
	}
	writeJSON(w, http.StatusOK, v)
}

// messages is the inbox: the story so far.
func (s *Server) messages(ctx context.Context, sess play.Session) ([]Message, error) {
	p := s.g.Pack().Manifest.Player
	type mail struct {
		id, from, subject, file string
		at                      time.Time
		show                    bool
	}
	all := []mail{
		{"calm", "Álvaro (fundador)", "Bem-vinda(o) ao Mercado Lerdo", p.Calm.Briefing, sess.StartedAt, sess.Phase != ""},
		{"growth", "#geral", "O tráfego subiu", p.Growth.Briefing, sess.IncidentAt, sess.Phase == play.Incident || sess.Phase == play.Completed},
		{"debrief", "Álvaro (fundador)", "Debrief: o que aconteceu", p.Debrief, sess.DoneAt, sess.Phase == play.Completed},
	}
	var out []Message
	for _, m := range all {
		if !m.show {
			continue
		}
		body, err := s.g.Document(ctx, m.file, Words)
		if err != nil {
			return nil, err
		}
		out = append(out, Message{ID: m.id, From: m.from, Subject: m.subject, At: m.at, Markdown: body})
	}
	return out, nil
}

// ---- quick actions ----

func (s *Server) next(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeError(w, http.StatusConflict, errBusy)
		return
	}
	started, err := s.g.Next(r.Context())
	if err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"started": started})
}

func (s *Server) hint(w http.ResponseWriter, _ *http.Request) {
	if !s.g.HasSession() {
		writeError(w, http.StatusConflict, game.ErrNoSession)
		return
	}
	h, ok, err := s.g.Hint()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"hint": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hint": HintView{Level: h.Level, Kind: h.Kind, Text: play.RenderBriefing(h.Text, Words)}})
}

func (s *Server) stop(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Down bool `json:"down"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if s.busy() {
		writeError(w, http.StatusConflict, errBusy)
		return
	}
	if err := s.g.Stop(r.Context(), body.Down); err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"down": body.Down})
}

func (s *Server) pulse(w http.ResponseWriter, r *http.Request) {
	span := 10 * time.Minute
	if v, err := strconv.Atoi(r.URL.Query().Get("span")); err == nil && v >= 60 && v <= 3600 {
		span = time.Duration(v) * time.Second
	}
	step := max(5*time.Second, span/120)
	p, err := s.g.Pulse(r.Context(), span, step)
	if err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) topology(w http.ResponseWriter, _ *http.Request) {
	if !s.g.HasSession() {
		writeError(w, http.StatusConflict, game.ErrNoSession)
		return
	}
	writeJSON(w, http.StatusOK, s.g.Topology())
}

// catalogEntry is a catalog page plus whether the workspace has it.
type catalogEntry struct {
	topology.Entry
	Installed bool `json:"installed"`
}

func (s *Server) catalog(w http.ResponseWriter, _ *http.Request) {
	var ws *topology.Topology
	if s.g.HasSession() {
		ws = s.g.Topology().Workspace
	}
	out := make([]catalogEntry, 0, len(topology.Catalog))
	for _, k := range []topology.Kind{topology.KindNginx, topology.KindApp, topology.KindPgBouncer, topology.KindPostgres} {
		e := catalogEntry{Entry: topology.Catalog[k], Installed: !topology.Catalog[k].Optional}
		if ws != nil {
			e.Installed = ws.Installed(k)
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) install(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind topology.Kind `json:"kind"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if s.busy() { // a deploy or test in progress reads topology.yaml
		writeError(w, http.StatusConflict, errBusy)
		return
	}
	switch err := s.g.Install(body.Kind); {
	case errors.Is(err, topology.ErrInstalled), errors.Is(err, game.ErrNoSession):
		writeError(w, http.StatusConflict, err)
	case err != nil:
		writeError(w, http.StatusBadRequest, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) profile(w http.ResponseWriter, _ *http.Request) {
	if !s.g.HasSession() {
		writeError(w, http.StatusConflict, game.ErrNoSession)
		return
	}
	writeJSON(w, http.StatusOK, s.g.Profile())
}

func (s *Server) campaign(w http.ResponseWriter, _ *http.Request) {
	v, err := s.g.Campaign()
	if errors.Is(err, game.ErrNotInCampaign) {
		writeJSON(w, http.StatusOK, map[string]any{"missions": []any{}})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) gitStatus(w http.ResponseWriter, r *http.Request) {
	if !s.g.HasSession() {
		writeError(w, http.StatusConflict, game.ErrNoSession)
		return
	}
	c, err := s.g.GitStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if c == nil {
		c = []game.FileChange{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"changes": c})
}

func (s *Server) gitDiff(w http.ResponseWriter, r *http.Request) {
	if !s.g.HasSession() {
		writeError(w, http.StatusConflict, game.ErrNoSession)
		return
	}
	d, err := s.g.GitDiff(r.Context(), r.URL.Query().Get("path"), r.URL.Query().Get("since") == "start")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": d})
}

func (s *Server) gitLog(w http.ResponseWriter, r *http.Request) {
	if !s.g.HasSession() {
		writeError(w, http.StatusConflict, game.ErrNoSession)
		return
	}
	l, err := s.g.GitLog(r.Context(), 40)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commits": l})
}

func (s *Server) gitCommit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Message string `json:"message"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if s.busy() {
		writeError(w, http.StatusConflict, errBusy)
		return
	}
	c, err := s.g.GitCommit(r.Context(), body.Message)
	switch {
	case errors.Is(err, game.ErrNothingToCommit):
		writeError(w, http.StatusConflict, err)
	case err != nil:
		writeError(w, http.StatusBadRequest, err)
	default:
		writeJSON(w, http.StatusOK, c)
	}
}

// viewed records the product events the CLI records when it prints the
// same thing (docs/product/playtest-1.md §6). Only known names are accepted.
func (s *Server) viewed(w http.ResponseWriter, r *http.Request) {
	var body struct {
		What string `json:"what"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.g.HasSession() {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	props := map[string]any{"view": "web", "phase": string(s.g.Session().Phase)}
	switch body.What {
	case "debrief":
		s.g.Event("debrief_viewed", props)
	case "pulse", "support":
		s.g.Event(map[string]string{"pulse": "status_viewed", "support": "support_viewed"}[body.What], props)
		if body.What == "pulse" {
			// The numbers are a nicety of the story; a store without data yet must not fail the view.
			if err := s.g.RecordBaseline(r.Context(), nil); err != nil {
				s.log.Warn("baseline not recorded", "err", err)
			}
		}
		if err := s.g.MarkViewed(body.What); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown view %q", body.What))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) tickets(w http.ResponseWriter, r *http.Request) {
	// A Production Test recreates the database under the story's traffic and
	// measures it: reconciling now would query that database and report its
	// reset as lost orders. Show what was true before the job started.
	if s.busy() {
		s.mu.Lock()
		t := s.lastTickets
		s.mu.Unlock()
		if t == nil {
			t = []game.Ticket{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"tickets": t})
		return
	}
	t, err := s.g.Tickets(r.Context())
	if err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	if t == nil {
		t = []game.Ticket{}
	}
	s.mu.Lock()
	s.lastTickets = t
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"tickets": t})
}

// ---- notes and preferences ----

func (s *Server) notesPath() string {
	return filepath.Join(play.StateDir(s.g.Session().Workspace), "notes.md")
}

func (s *Server) getNotes(w http.ResponseWriter, _ *http.Request) {
	if !s.g.HasSession() {
		writeJSON(w, http.StatusOK, map[string]string{"text": ""})
		return
	}
	b, err := os.ReadFile(s.notesPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"text": string(b)})
}

func (s *Server) putNotes(w http.ResponseWriter, r *http.Request) {
	// The notes live in the workspace, which exists only once the game started.
	if !s.g.HasSession() {
		writeError(w, http.StatusConflict, game.ErrNoSession)
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxNotesBody)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := writeFileAtomic(s.notesPath(), []byte(body.Text)); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getPrefs(w http.ResponseWriter, _ *http.Request) {
	b, err := os.ReadFile(s.prefs)
	if errors.Is(err, os.ErrNotExist) || (err == nil && !json.Valid(b)) {
		b = []byte("{}")
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

// putPrefs stores the UI's preferences as an opaque JSON object: the UI owns
// their shape and falls back to defaults for anything it does not recognize.
func (s *Server) putPrefs(w http.ResponseWriter, r *http.Request) {
	var prefs map[string]any
	if err := readJSON(w, r, &prefs); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	b, err := json.Marshal(prefs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.prefs), 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := writeFileAtomic(s.prefs, b); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DefaultPrefsPath is $XDG_CONFIG_HOME/sesim/ui.json.
func DefaultPrefsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sesim", "ui.json"), nil
}

// ---- helpers ----

var errBusy = errors.New("outra operação ainda está em andamento")

func statusOf(err error) int {
	switch {
	case errors.Is(err, game.ErrNotRunning), errors.Is(err, game.ErrNoSession):
		return http.StatusConflict
	case errors.Is(err, context.Canceled):
		return 499 // client went away
	}
	return http.StatusInternalServerError
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	err := json.NewDecoder(r.Body).Decode(v)
	if errors.Is(err, io.EOF) {
		return nil // an empty body means the defaults
	}
	return err
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "encode: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(b)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func writeFileAtomic(p string, b []byte) error {
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
