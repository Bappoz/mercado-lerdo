import { FitAddon } from "@xterm/addon-fit";
import { Terminal as XTerm } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { useEffect, useRef, useState } from "react";
import { token } from "../api.ts";

type Status = "connecting" | "open" | "closed";

function themeFromCss(el: HTMLElement) {
  const css = getComputedStyle(el);
  const v = (name: string) => css.getPropertyValue(name).trim();
  return {
    font: v("--font-mono"),
    theme: {
      background: v("--term-bg"),
      foreground: v("--term-ink"),
      cursor: v("--term-accent"),
      selectionBackground: v("--term-dim"),
    },
  };
}

/**
 * A real shell in the toolbox container, on the store's network (psql is
 * already connected). The server bridges it over a WebSocket (ADR-0008).
 */
export function Terminal({ running, themeKey }: { running: boolean; themeKey: string }) {
  const [attempt, setAttempt] = useState(0);
  if (!running)
    return <p className="empty">A loja está desligada: não há onde abrir o terminal.</p>;
  // A new key is a new session: the old one is torn down, a new shell starts.
  return <Session key={attempt} themeKey={themeKey} onReopen={() => setAttempt((a) => a + 1)} />;
}

function Session({ themeKey, onReopen }: { themeKey: string; onReopen: () => void }) {
  const host = useRef<HTMLDivElement>(null);
  const term = useRef<XTerm | null>(null);
  const [status, setStatus] = useState<Status>("connecting");

  useEffect(() => {
    const el = host.current;
    if (!el) return;
    const look = themeFromCss(el);
    const t = new XTerm({
      fontFamily: look.font,
      fontSize: 14,
      theme: look.theme,
      cursorBlink: true,
      scrollback: 5000,
    });
    const fit = new FitAddon();
    t.loadAddon(fit);
    t.open(el);
    term.current = t;
    fit.fit();

    const ws = new WebSocket(
      `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/ws/terminal`,
    );
    ws.binaryType = "arraybuffer";
    const enc = new TextEncoder();
    const sendSize = () => {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ cols: t.cols, rows: t.rows }));
    };
    setStatus("connecting");
    ws.onopen = () => {
      ws.send(token); // first message authenticates (threat model T3)
      sendSize();
      setStatus("open");
      t.focus();
    };
    ws.onmessage = (e) => {
      if (e.data instanceof ArrayBuffer) t.write(new Uint8Array(e.data));
    };
    ws.onclose = () => {
      setStatus("closed");
      t.write("\r\n\x1b[2m[sessão encerrada]\x1b[0m\r\n");
    };
    const onData = t.onData((d) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(enc.encode(d));
    });
    const onResize = t.onResize(sendSize);
    const ro = new ResizeObserver(() => fit.fit());
    ro.observe(el);

    return () => {
      ro.disconnect();
      onData.dispose();
      onResize.dispose();
      ws.close();
      t.dispose();
      term.current = null;
    };
  }, []);

  // Follow theme changes without reconnecting.
  useEffect(() => {
    const el = host.current;
    if (!el || !term.current || !themeKey) return;
    const look = themeFromCss(el);
    term.current.options.theme = look.theme;
    term.current.options.fontFamily = look.font;
  }, [themeKey]);

  return (
    <div className="term">
      <div className="term-host" ref={host} />
      {status === "closed" && (
        <div className="term-bar">
          <span>Sessão encerrada.</span>
          <button type="button" className="btn small" onClick={onReopen}>
            Abrir outra
          </button>
        </div>
      )}
    </div>
  );
}
