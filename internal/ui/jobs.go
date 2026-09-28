package ui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/game"
	"github.com/Bappoz/software-engineering-simulator/internal/stack"
)

// Job is one long operation (start the game, Quick Check, Production Test).
// The UI polls it through /api/state.
type Job struct {
	ID       int               `json:"id"`
	Kind     string            `json:"kind"`
	Running  bool              `json:"running"`
	Lines    []string          `json:"lines"`
	Error    string            `json:"error,omitempty"`
	Started  time.Time         `json:"started"`
	Finished time.Time         `json:"finished,omitzero"`
	Check    *game.CheckResult `json:"check,omitempty"`
	Test     *game.TestResult  `json:"test,omitempty"`
	Deploy   []stack.Change    `json:"deploy,omitempty"`
	Lab      *game.LabResult   `json:"lab,omitempty"`

	labParams game.LabParams // the experiment a "lab" job runs
	canonical bool           // how an "advance" job enters the next scenario
}

const maxJobLines = 200

func (s *Server) busy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.job != nil && s.job.Running
}

// currentJob returns a copy of the current (or last) job.
func (s *Server) currentJob() *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job == nil {
		return nil
	}
	j := *s.job
	// Never nil: an empty log is [] in JSON, not null.
	j.Lines = append(make([]string, 0, len(s.job.Lines)), s.job.Lines...)
	return &j
}

// say appends a line to the running job.
func (s *Server) say(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job == nil || !s.job.Running {
		return
	}
	s.job.Lines = append(s.job.Lines, line)
	if n := len(s.job.Lines); n > maxJobLines {
		s.job.Lines = s.job.Lines[n-maxJobLines:]
	}
}

func (s *Server) startJob(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if kind != "play" && !s.g.HasSession() {
			writeError(w, http.StatusConflict, game.ErrNoSession)
			return
		}
		var lab game.LabParams
		var adv struct {
			Canonical bool `json:"canonical"`
		}
		if kind == "advance" {
			if err := readJSON(w, r, &adv); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
		if kind == "lab" {
			if err := readJSON(w, r, &lab); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if err := lab.Validate(); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
		s.mu.Lock()
		if s.job != nil && s.job.Running {
			s.mu.Unlock()
			writeError(w, http.StatusConflict, errBusy)
			return
		}
		s.nextJob++
		j := &Job{ID: s.nextJob, Kind: kind, Running: true, Lines: []string{}, Started: time.Now().UTC(), labParams: lab, canonical: adv.Canonical}
		s.job = j
		s.mu.Unlock()
		go s.run(j)
		writeJSON(w, http.StatusAccepted, s.currentJob())
	}
}

// run executes j on the server's context: closing the browser tab does not
// abort a Production Test, stopping `sesim ui` does.
func (s *Server) run(j *Job) {
	ctx := s.ctx
	var (
		check  *game.CheckResult
		test   *game.TestResult
		deploy []stack.Change
		lab    *game.LabResult
		err    error
	)
	switch j.Kind {
	case "play":
		if _, err = s.g.Start(ctx, s.say); err == nil {
			err = s.g.WatchStats(ctx)
		}
	case "check":
		var r game.CheckResult
		if r, err = s.g.Check(ctx, s.say); err == nil {
			check = &r
		}
	case "test":
		var r game.TestResult
		if r, err = s.g.Test(ctx, 0, s.say); err == nil {
			test = &r
		}
	case "deploy":
		if deploy, err = s.g.Deploy(ctx, s.say); err == nil && deploy == nil {
			deploy = []stack.Change{}
		}
	case "advance":
		_, err = s.g.Advance(ctx, j.canonical, s.say)
	case "lab":
		var r game.LabResult
		if r, err = s.g.Lab(ctx, j.labParams, s.say); err == nil {
			lab = &r
		}
	default:
		err = fmt.Errorf("operação desconhecida %q", j.Kind)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j.Running, j.Finished, j.Check, j.Test, j.Deploy, j.Lab = false, time.Now().UTC(), check, test, deploy, lab
	if err != nil {
		j.Error = err.Error()
		if !errors.Is(err, context.Canceled) {
			s.log.Warn("job failed", "kind", j.Kind, "err", err)
		}
	}
}

// JobLog turns the game's progress log (sandbox steps, Production Test
// runs) into lines of the running job.
type JobLog struct {
	mu sync.Mutex
	s  *Server
}

// Attach routes the log to s; records before that are dropped.
func (h *JobLog) Attach(s *Server) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.s = s
}

// Enabled implements slog.Handler.
func (h *JobLog) Enabled(_ context.Context, l slog.Level) bool { return l >= slog.LevelInfo }

// Handle implements slog.Handler.
func (h *JobLog) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	s := h.s
	h.mu.Unlock()
	if s != nil {
		s.say(describe(r))
	}
	return nil
}

// WithAttrs implements slog.Handler; the game's loggers carry no preset attributes.
func (h *JobLog) WithAttrs([]slog.Attr) slog.Handler { return h }

// WithGroup implements slog.Handler.
func (h *JobLog) WithGroup(string) slog.Handler { return h }

// stepWords are the sandbox's steps as the player reads them.
var stepWords = map[string]string{
	"create":         "criar a sandbox",
	"build app":      "compilar o app",
	"start postgres": "subir o Postgres",
	"seed template":  "preparar o banco original",
	"reset data":     "restaurar os dados",
	"start services": "subir os serviços",
}

// logWords are the other records a job can show; the logs themselves stay in English.
var logWords = map[string]string{
	"collected orphaned sandboxes":                          "Sandboxes órfãs removidas",
	"db cpu query failed":                                   "Não consegui medir a CPU do banco nesta execução",
	"db probe failed":                                       "O teste de velocidade do banco falhou nesta execução",
	"could not restart the story's traffic":                 "Não consegui religar o tráfego do dia",
	"could not reload the session saved by another process": "Não consegui recarregar o progresso salvo por outra janela",
}

// describe words the records the player sees while waiting.
func describe(r slog.Record) string {
	attrs := map[string]string{}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	switch r.Message {
	case "sandbox step":
		step := attrs["step"]
		if w, ok := stepWords[step]; ok {
			step = w
		}
		return fmt.Sprintf("ok: %s (%s ms)", step, attrs["ms"])
	case "production test run":
		return fmt.Sprintf("Execução %s de %s…", attrs["run"], attrs["of"])
	case "deploy":
		return attrs["step"]
	case "fault":
		return fmt.Sprintf("Derrubando %s (aos %s)…", attrs["crash"], attrs["after"])
	}
	var b strings.Builder
	if w, ok := logWords[r.Message]; ok {
		b.WriteString(w)
	} else {
		b.WriteString(r.Message)
	}
	for k, v := range attrs {
		if k == "run" || k == "id" {
			continue // internal identifiers
		}
		fmt.Fprintf(&b, " %s=%s", k, v)
	}
	return b.String()
}
