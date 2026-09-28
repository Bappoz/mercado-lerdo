package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// terminal bridges a WebSocket to a shell in the toolbox container.
// Protocol: the first text message is the token; after it, binary messages
// are keystrokes and text messages are control ({"cols":80,"rows":24}); the
// server sends the terminal's output as binary messages.
func (s *Server) terminal(w http.ResponseWriter, r *http.Request) {
	// Accept rejects cross-origin handshakes: the Origin must be the Host (T3).
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = c.CloseNow() }()
	c.SetReadLimit(64 << 10)
	ctx := r.Context()

	actx, cancel := context.WithTimeout(ctx, 5*time.Second)
	typ, b, err := c.Read(actx)
	cancel()
	if err != nil || typ != websocket.MessageText || !s.validToken(string(b)) {
		_ = c.Close(websocket.StatusPolicyViolation, "não autorizado")
		return
	}
	tty, err := s.gm().Terminal(ctx, 80, 24)
	if err != nil {
		_ = c.Close(websocket.StatusInternalError, truncate(err.Error(), 120))
		return
	}
	s.gm().Event("shell_opened", map[string]any{"view": "web"})
	if err := s.gm().MarkViewed("shell"); err != nil {
		s.log.Warn("shell view not recorded", "err", err)
	}

	var wg sync.WaitGroup
	defer wg.Wait()
	defer tty.Close() // runs first: unblocks the reader below

	wg.Go(func() {
		buf := make([]byte, 32<<10)
		for {
			n, err := tty.Read(buf)
			if n > 0 {
				if werr := c.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				_ = c.Close(websocket.StatusNormalClosure, "o shell terminou")
				return
			}
		}
	})
	for {
		typ, b, err := c.Read(ctx)
		if err != nil {
			return
		}
		switch typ {
		case websocket.MessageBinary:
			if _, err := tty.Write(b); err != nil {
				return
			}
		case websocket.MessageText:
			var size struct{ Cols, Rows uint }
			if json.Unmarshal(b, &size) == nil && size.Cols > 0 && size.Cols <= 1000 && size.Rows > 0 && size.Rows <= 500 {
				_ = tty.Resize(ctx, size.Cols, size.Rows)
			}
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "")
}
