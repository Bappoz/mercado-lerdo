package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/Bappoz/software-engineering-simulator/internal/ui"
	"github.com/Bappoz/software-engineering-simulator/web"
)

// runUI: sesim ui — LerdOS, the game in the browser.
func runUI(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	ptFlags(fs, stderr)
	var f gameFlags
	f.register(fs)
	addr := fs.String("addr", "127.0.0.1:0", "endereço local em que o LerdOS escuta (porta 0 = qualquer porta livre)")
	noOpen := fs.Bool("no-open", false, "não abre o navegador")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	prefs, err := ui.DefaultPrefsPath()
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return 1
	}
	jobLog := &ui.JobLog{}
	g, err := f.open(ctx, slog.New(jobLog))
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return 1
	}
	defer g.Close()
	log := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	srv, err := ui.New(ctx, g, web.Dist(), prefs, log)
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return 1
	}
	jobLog.Attach(srv)
	if g.Running() {
		if err := g.WatchStats(ctx); err != nil {
			log.Warn("container stats unavailable; the dashboard will miss CPU", "err", err)
		}
	}
	ln, err := srv.Listen(*addr)
	if err != nil {
		fmt.Fprintln(stderr, "sesim:", err)
		return 1
	}
	url := fmt.Sprintf("http://%s/#t=%s", ln.Addr(), srv.Token())
	fmt.Fprintf(stdout, "LerdOS em %s\n(o endereço tem a chave desta sessão: não compartilhe. Ctrl+C encerra; a loja continua rodando.)\n", url)
	if !*noOpen {
		openBrowser(url)
	}
	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(sctx)
	}()
	if err := hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(stderr, "sesim:", err)
		return 1
	}
	return 0
}

// openBrowser is best effort: the URL is printed anyway.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url) //nolint:gosec,noctx // G204: fixed binary; the browser outlives the command's context
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url) //nolint:gosec,noctx // G204: fixed binary
	default:
		cmd = exec.Command("xdg-open", url) //nolint:gosec,noctx // G204: fixed binary
	}
	if cmd.Start() == nil {
		go func() { _ = cmd.Wait() }()
	}
}
