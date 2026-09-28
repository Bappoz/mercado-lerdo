package ui

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
)

func testServer(t *testing.T, static fstest.MapFS) *httptest.Server {
	t.Helper()
	s := &Server{token: "secret-token", static: static, log: slog.New(slog.DiscardHandler), ctx: context.Background()}
	ts := httptest.NewServer(nil)
	ts.Config.Handler = s.Handler()
	s.hosts = map[string]bool{strings.TrimPrefix(ts.URL, "http://"): true}
	t.Cleanup(ts.Close)
	return ts
}

type reply struct {
	code   int
	header http.Header
	body   string
}

func do(t *testing.T, method, url string, header map[string]string) reply {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return reply{code: resp.StatusCode, header: resp.Header, body: string(b)}
}

func TestGuard(t *testing.T) {
	ts := testServer(t, fstest.MapFS{"index.html": {Data: []byte("<html>os</html>")}})
	tests := []struct {
		name   string
		method string
		path   string
		header map[string]string
		want   int
	}{
		{"spa", "GET", "/", nil, 200},
		{"client route falls back to index", "GET", "/apps/pulse", nil, 200},
		{"foreign host (DNS rebinding)", "GET", "/", map[string]string{"Host": "evil.example:80"}, 403},
		{"api without token", "GET", "/api/state", nil, 401},
		{"api with wrong token", "GET", "/api/state", map[string]string{"Authorization": "Bearer nope"}, 401},
		{"api token in the wrong scheme", "GET", "/api/state", map[string]string{"Authorization": "secret-token"}, 401},
		{"post to a static path", "POST", "/", nil, 405},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := do(t, tt.method, ts.URL+tt.path, tt.header)
			if resp.code != tt.want {
				t.Fatalf("status = %d, want %d", resp.code, tt.want)
			}
			if tt.want == 200 {
				csp := resp.header.Get("Content-Security-Policy")
				if !strings.Contains(csp, "script-src 'self'") || !strings.Contains(csp, "frame-ancestors 'none'") {
					t.Errorf("CSP = %q", csp)
				}
				if resp.body != "<html>os</html>" {
					t.Errorf("body = %q", resp.body)
				}
			}
		})
	}
}

func TestNotBuilt(t *testing.T) {
	ts := testServer(t, fstest.MapFS{".gitkeep": {}})
	if resp := do(t, "GET", ts.URL+"/", nil); resp.code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.code)
	}
}

func TestListenLoopbackOnly(t *testing.T) {
	s := &Server{ctx: context.Background()}
	for _, addr := range []string{"0.0.0.0:0", "192.0.2.1:0", "[::1]:0", "localhost:0"} {
		if ln, err := s.Listen(addr); err == nil {
			_ = ln.Close()
			t.Errorf("Listen(%s) succeeded, want refusal", addr)
		}
	}
	ln, err := s.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if len(s.hosts) != 2 {
		t.Errorf("hosts = %v", s.hosts)
	}
}

func TestTerminalRejects(t *testing.T) {
	ts := testServer(t, fstest.MapFS{})
	ws := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/terminal"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("cross-origin handshake", func(t *testing.T) {
		c, resp, err := websocket.Dial(ctx, ws, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"http://evil.example"}}})
		if err == nil {
			_ = c.CloseNow()
			t.Fatal("dial succeeded, want a refused handshake")
		}
		if resp != nil {
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("status = %d, want 403", resp.StatusCode)
			}
		}
	})
	t.Run("wrong token", func(t *testing.T) {
		c, resp, err := websocket.Dial(ctx, ws, nil)
		if err != nil {
			t.Fatal(err)
		}
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		defer func() { _ = c.CloseNow() }()
		if err := c.Write(ctx, websocket.MessageText, []byte("nope")); err != nil {
			t.Fatal(err)
		}
		_, _, err = c.Read(ctx)
		if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
			t.Fatalf("read err = %v, want close 1008", err)
		}
	})
}

func TestDescribe(t *testing.T) {
	rec := func(msg string, attrs ...any) slog.Record {
		r := slog.NewRecord(time.Time{}, slog.LevelInfo, msg, 0)
		r.Add(attrs...)
		return r
	}
	for _, tt := range []struct {
		r    slog.Record
		want string
	}{
		{rec("sandbox step", "run", "r1", "step", "build app", "ms", 1200), "ok: compilar o app (1200 ms)"},
		{rec("production test run", "run", 2, "of", 3, "id", "x"), "Execução 2 de 3…"},
		{rec("db cpu query failed", "run", 1), "Não consegui medir a CPU do banco nesta execução"},
		{rec("something new", "k", "v"), "something new k=v"},
	} {
		if got := describe(tt.r); got != tt.want {
			t.Errorf("describe = %q, want %q", got, tt.want)
		}
	}
}

func TestJobLinesNeverNull(t *testing.T) {
	s := &Server{job: &Job{ID: 1, Kind: "deploy", Running: true, Lines: []string{}}}
	b, err := json.Marshal(s.currentJob())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"lines":[]`) {
		t.Errorf("job JSON = %s, want an empty lines array", b)
	}
}
