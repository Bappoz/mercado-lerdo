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
	j.Lines = append([]string(nil), s.job.Lines...)
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
	return func(w http.ResponseWriter, _ *http.Request) {
		if kind != "play" && !s.g.HasSession() {
			writeError(w, http.StatusConflict, game.ErrNoSession)
			return
		}
		s.mu.Lock()
		if s.job != nil && s.job.Running {
			s.mu.Unlock()
			writeError(w, http.StatusConflict, errBusy)
			return
		}
		s.nextJob++
		j := &Job{ID: s.nextJob, Kind: kind, Running: true, Lines: []string{}, Started: time.Now().UTC()}
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
		check *game.CheckResult
		test  *game.TestResult
		err   error
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
	default:
		err = fmt.Errorf("unknown job %q", j.Kind)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j.Running, j.Finished, j.Check, j.Test = false, time.Now().UTC(), check, test
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

// describe words the records the player sees while waiting.
func describe(r slog.Record) string {
	attrs := map[string]string{}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	switch r.Message {
	case "sandbox step":
		return fmt.Sprintf("ok: %s (%s ms)", attrs["step"], attrs["ms"])
	case "production test run":
		return fmt.Sprintf("Execução %s de %s…", attrs["run"], attrs["of"])
	}
	var b strings.Builder
	b.WriteString(r.Message)
	for k, v := range attrs {
		if k == "run" || k == "id" {
			continue // internal identifiers
		}
		fmt.Fprintf(&b, " %s=%s", k, v)
	}
	return b.String()
}
