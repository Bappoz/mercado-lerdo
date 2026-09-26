import {
  type CSSProperties,
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { api } from "./api.ts";
import { Architecture } from "./apps/Architecture.tsx";
import { Deploy } from "./apps/Deploy.tsx";
import { Docs } from "./apps/Docs.tsx";
import { Inbox, messageKey } from "./apps/Inbox.tsx";
import { Notes } from "./apps/Notes.tsx";
import { Pulse } from "./apps/Pulse.tsx";
import { Settings } from "./apps/Settings.tsx";
import { Tests } from "./apps/Tests.tsx";
import { AppBoundary } from "./os/AppBoundary.tsx";
import { AppIcon } from "./os/pixel.tsx";
import { Tabby } from "./os/Tabby.tsx";
import { useWindows, Window } from "./os/windows.tsx";
import {
  type AppId,
  accentOf,
  cssVars,
  DEFAULT_PREFS,
  normalizePrefs,
  type Prefs,
  themeOf,
} from "./theme.ts";
import { useGame } from "./useGame.ts";

const APPS: { id: AppId; title: string; label: string; reading?: boolean }[] = [
  { id: "inbox", title: "Caixa de entrada", label: "E-mail", reading: true },
  { id: "pulse", title: "Pulse · produção agora", label: "Pulse" },
  { id: "terminal", title: "Terminal · rede da Nexora", label: "Terminal" },
  { id: "tests", title: "Testes", label: "Testes", reading: true },
  { id: "architecture", title: "Arquitetura · topology.yaml", label: "Arquitetura", reading: true },
  { id: "deploy", title: "Deploy", label: "Deploy", reading: true },
  { id: "docs", title: "Docs · catálogo de componentes", label: "Docs", reading: true },
  { id: "notes", title: "Notas", label: "Notas", reading: true },
  { id: "settings", title: "Configurações", label: "Config.", reading: true },
];

// xterm.js is most of the bundle: load it when the terminal first opens.
const Terminal = lazy(() => import("./apps/Terminal.tsx").then((m) => ({ default: m.Terminal })));

const PHASE = { calm: "dia calmo", incident: "incidente", completed: "resolvido" } as const;

/** usePrefs loads the saved preferences; nothing is saved before they load,
 * so an early write can never replace them with the defaults. */
function usePrefs(onError: (msg: string) => void): [Prefs, (p: Prefs) => void, boolean] {
  const [prefs, setPrefs] = useState<Prefs>(DEFAULT_PREFS);
  const [loaded, setLoaded] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => {
    // Unreadable preferences are not an error worth interrupting for: the defaults apply.
    api<unknown>("GET", "/api/prefs")
      .then(
        (p) => setPrefs(normalizePrefs(p)),
        () => setPrefs(DEFAULT_PREFS),
      )
      .finally(() => setLoaded(true));
  }, []);
  const save = useCallback(
    (p: Prefs) => {
      setPrefs(p);
      if (!loaded) return;
      clearTimeout(timer.current);
      timer.current = setTimeout(() => {
        api("PUT", "/api/prefs", p).catch((e: unknown) =>
          onError(`Não consegui salvar as preferências: ${e instanceof Error ? e.message : e}`),
        );
      }, 300);
    },
    [onError, loaded],
  );
  return [prefs, save, loaded];
}

function Clock() {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 15_000);
    return () => clearInterval(id);
  }, []);
  return <time>{now.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" })}</time>;
}

export function App() {
  const game = useGame();
  const [notice, setToast] = useState<string | null>(null);
  const [prefs, setPrefs, prefsLoaded] = usePrefs(setToast);
  const wm = useWindows();
  const { state } = game;
  const toast = notice ?? game.error;

  const markSeen = useCallback(
    (key: string) => {
      if (prefsLoaded && !prefs.seen.includes(key))
        setPrefs({ ...prefs, seen: [...prefs.seen, key].slice(-50) });
    },
    [prefs, setPrefs, prefsLoaded],
  );

  // Announce jobs this page saw running when they finish, and open what they produced.
  const watching = useRef<number | null>(null);
  const job = state?.job;
  const { open } = wm;
  useEffect(() => {
    if (!job) return;
    if (job.running) {
      watching.current = job.id;
      return;
    }
    if (watching.current !== job.id) return;
    watching.current = null;
    if (job.error) setToast("Não deu certo: veja os detalhes no app Testes.");
    else if (job.kind === "play") {
      setToast("A Nexora está no ar.");
      open("pulse");
    } else if (job.kind === "check") setToast("Quick Check pronto.");
    else if (job.kind === "deploy") setToast("Deploy feito.");
    else setToast(job.test?.passed ? "Production Test: PASS!" : "Production Test terminou.");
    if (job.kind === "check" || job.kind === "test") open("tests");
    if (job.kind === "deploy") open("deploy");
  }, [job, open]);

  // A new mail (the incident, the debrief) pops the inbox up.
  const mails = state?.messages.length;
  const prevMails = useRef<number | undefined>(undefined);
  useEffect(() => {
    if (mails === undefined) return;
    if (prevMails.current !== undefined && mails > prevMails.current) {
      open("inbox");
      setToast("Nova mensagem na Caixa de entrada.");
    }
    prevMails.current = mails;
  }, [mails, open]);

  const { clearError } = game;
  useEffect(() => {
    if (!toast) return;
    const id = setTimeout(() => {
      setToast(null);
      clearError();
    }, 6000);
    return () => clearTimeout(id);
  }, [toast, clearError]);

  const theme = themeOf(prefs);
  const accent = accentOf(prefs);
  const style = cssVars(prefs) as CSSProperties;
  const scene = prefs.wallpaper === "cena" ? theme.scene : "none";

  if (game.unauthorized) {
    return (
      <div className="os locked" style={style}>
        <div className="lock">
          <h1>NexoraOS</h1>
          <p>
            Este endereço não tem a chave da sessão. Abra o link que o <code>sesim ui</code> mostrou
            no terminal.
          </p>
        </div>
      </div>
    );
  }

  const busy = state?.job?.running ?? false;
  const unread = state?.session
    ? state.messages.filter((m) => !prefs.seen.includes(messageKey(m))).length
    : state
      ? 1
      : 0;
  const body = (id: AppId) => {
    if (!state) return <p className="empty">Ligando…</p>;
    switch (id) {
      case "inbox":
        return (
          <Inbox
            state={state}
            seen={prefs.seen}
            onSeen={markSeen}
            busy={busy}
            onStart={() => void game.act(() => api("POST", "/api/play"))}
            onNext={() => void game.act(() => api("POST", "/api/next"))}
          />
        );
      case "pulse":
        return <Pulse running={state.running} />;
      case "terminal":
        return (
          <Suspense fallback={<p className="empty">Abrindo o terminal…</p>}>
            <Terminal running={state.running} themeKey={`${prefs.theme}/${prefs.chrome}`} />
          </Suspense>
        );
      case "tests":
        return (
          <Tests
            state={state}
            busy={busy}
            onCheck={() => void game.act(() => api("POST", "/api/check"))}
            onTest={() => void game.act(() => api("POST", "/api/test"))}
          />
        );
      case "architecture":
        return (
          <Architecture
            hasSession={state.session !== null}
            jobVersion={state.job && !state.job.running ? state.job.id : 0}
            onOpen={open}
          />
        );
      case "deploy":
        return (
          <Deploy
            state={state}
            busy={busy}
            onDeploy={() => void game.act(() => api("POST", "/api/deploy"))}
          />
        );
      case "docs":
        return (
          <Docs
            hasSession={state.session !== null}
            onInstalled={(name) =>
              setToast(`${name} está no topology.yaml. Faça um deploy para ligar.`)
            }
          />
        );
      case "notes":
        return <Notes hasSession={state.session !== null} />;
      case "settings":
        return (
          <Settings
            prefs={prefs}
            setPrefs={setPrefs}
            state={state}
            busy={busy}
            onStop={(down) => void game.act(() => api("POST", "/api/stop", { down }))}
          />
        );
    }
  };

  return (
    <div
      className="os"
      style={style}
      data-theme={theme.id}
      data-scene={scene}
      data-chrome={prefs.chrome}
      data-motion={prefs.motion ? "on" : "off"}
    >
      {scene === "night" && (
        <div className="scene night" aria-hidden="true">
          <span className="moon" />
          <span className="stars" />
        </div>
      )}
      {scene === "pastel" && (
        <div className="scene pastel" aria-hidden="true">
          <span className="cloud c1" />
          <span className="cloud c2" />
          <span className="cloud c3" />
        </div>
      )}

      <header className="topbar">
        <span className="logo">NexoraOS</span>
        <span className="topbar-title">{state?.scenario.title ?? ""}</span>
        <span className="topbar-status">
          {state?.session && (
            <span className={`pill ${state.session.phase}`}>{PHASE[state.session.phase]}</span>
          )}
          {state && (
            <span className={state.running ? "pill live" : "pill"}>
              {state.running ? "Nexora no ar" : "Nexora desligada"}
            </span>
          )}
          {busy && <span className="pill busy">trabalhando…</span>}
        </span>
        <Clock />
      </header>

      {APPS.map((a) => (
        <Window
          key={a.id}
          app={a.id}
          title={a.title}
          frame={wm.frames[a.id]}
          compact={wm.compact}
          reading={a.reading}
          titleBar={theme.titleBar(accent, a.id)}
          wm={wm}
        >
          <AppBoundary name={a.label}>{body(a.id)}</AppBoundary>
        </Window>
      ))}

      <Tabby state={state} busy={game.act} onOpen={open} motion={prefs.motion} />

      <nav className="dock" aria-label="Apps">
        {APPS.map((a) => (
          <button
            type="button"
            key={a.id}
            className="dock-item"
            aria-label={a.title}
            aria-pressed={wm.frames[a.id].open}
            onClick={() => wm.toggle(a.id)}
          >
            <AppIcon app={a.id} />
            <span className="dock-label">{a.label}</span>
            {a.id === "inbox" && unread > 0 && <span className="badge">{unread}</span>}
            {wm.frames[a.id].open && <span className="dock-dot" aria-hidden="true" />}
          </button>
        ))}
      </nav>

      {toast && (
        <div className="toast" role="status">
          {toast}
        </div>
      )}
    </div>
  );
}
