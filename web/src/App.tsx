import {
  type CSSProperties,
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { api, type ClockView, type Deadline } from "./api.ts";
import { Architecture } from "./apps/Architecture.tsx";
import { Deploy } from "./apps/Deploy.tsx";
import { Docs } from "./apps/Docs.tsx";
import { Git } from "./apps/Git.tsx";
import { Inbox, messageKey } from "./apps/Inbox.tsx";
import { Lab } from "./apps/Lab.tsx";
import { Missions } from "./apps/Missions.tsx";
import { Notes } from "./apps/Notes.tsx";
import { Profile } from "./apps/Profile.tsx";
import { Pulse } from "./apps/Pulse.tsx";
import { Settings } from "./apps/Settings.tsx";
import { Support } from "./apps/Support.tsx";
import { Tests } from "./apps/Tests.tsx";
import { Intro } from "./menu/Intro.tsx";
import { Menu } from "./menu/Menu.tsx";
import { money } from "./money.ts";
import { AppBoundary } from "./os/AppBoundary.tsx";
import { AppIcon } from "./os/pixel.tsx";
import { Slotthy } from "./os/Slotthy.tsx";
import { Tour } from "./os/Tour.tsx";
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
  { id: "missions", title: "Missões · Stage 1", label: "Missões", reading: true },
  { id: "inbox", title: "Caixa de entrada", label: "E-mail", reading: true },
  { id: "pulse", title: "Pulse · produção agora", label: "Pulse" },
  { id: "terminal", title: "Terminal · rede da loja", label: "Terminal" },
  { id: "tests", title: "Testes", label: "Testes", reading: true },
  { id: "support", title: "Suporte · reclamações", label: "Suporte", reading: true },
  { id: "architecture", title: "Arquitetura · topology.yaml", label: "Arquitetura", reading: true },
  { id: "deploy", title: "Deploy", label: "Deploy", reading: true },
  { id: "git", title: "Git · workspace", label: "Git", reading: true },
  { id: "lab", title: "Laboratório de carga", label: "Lab", reading: true },
  { id: "docs", title: "Docs · leituras e componentes", label: "Docs", reading: true },
  { id: "notes", title: "Notas", label: "Notas", reading: true },
  { id: "profile", title: "Perfil · carreira", label: "Perfil", reading: true },
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

/** Countdown shows how long is left to answer the story's most urgent request. */
function Countdown({
  deadline,
  clock,
  onOpen,
}: {
  deadline: Deadline;
  clock: ClockView;
  onOpen: () => void;
}) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);
  // With the clock stopped, the countdown stands where it was (ADR-0026).
  const at = clock.paused && clock.since ? new Date(clock.since).getTime() : now;
  const left = Math.max(0, new Date(deadline.until).getTime() - at);
  const mm = String(Math.floor(left / 60000)).padStart(2, "0");
  const ss = String(Math.floor((left % 60000) / 1000)).padStart(2, "0");
  const label = deadline.kind === "emergencia" ? "Emergência" : "Pedido";
  return (
    <button
      type="button"
      className={deadline.kind === "emergencia" ? "pill emergency" : "pill request"}
      title={`${deadline.from}: ${deadline.title}`}
      onClick={onOpen}
    >
      {label} · {mm}:{ss}
      {clock.paused && " ⏸"}
    </button>
  );
}

function Clock() {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 15_000);
    return () => clearInterval(id);
  }, []);
  return <time>{now.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" })}</time>;
}

/** Where the player is: the title screen, the intro cutscene, or LerdOS itself. */
type Screen = { at: "menu" } | { at: "intro"; next: "menu" | "desktop" } | { at: "desktop" };

export function App() {
  const game = useGame();
  const [screen, setScreen] = useState<Screen>({ at: "menu" });
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

  // The first-day training: shown on a first visit (no game yet, never seen),
  // and again when the player asks for it in Settings.
  const [touring, setTouring] = useState(false);
  const showTour =
    touring || (prefsLoaded && !prefs.tour && state !== null && state.session === null);
  const openApp = wm.open; // stable (useCallback in useWindows)
  const endTour = useCallback(() => {
    setTouring(false);
    if (!prefs.tour) setPrefs({ ...prefs, tour: true });
    openApp("inbox");
  }, [prefs, setPrefs, openApp]);

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
      setToast("A loja está no ar.");
      open("pulse");
    } else if (job.kind === "check") setToast("Quick Check pronto.");
    else if (job.kind === "deploy") setToast("Deploy feito.");
    else if (job.kind === "lab") setToast("Experimento terminado.");
    else if (job.kind === "advance") {
      setToast("Próximo cenário pronto. Ligue a loja pela Caixa de entrada.");
      open("inbox");
    } else setToast(job.test?.passed ? "Production Test: PASS!" : "Production Test terminou.");
    if (job.kind === "check" || job.kind === "test") open("tests");
    if (job.kind === "deploy") open("deploy");
    if (job.kind === "lab") open("lab");
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
          <h1>LerdOS</h1>
          <p>
            Este endereço não tem a chave da sessão. Abra o link que o <code>sesim ui</code> mostrou
            no terminal.
          </p>
        </div>
      </div>
    );
  }

  if (screen.at !== "desktop") {
    return (
      <div
        className="os"
        style={style}
        data-theme={theme.id}
        data-motion={prefs.motion ? "on" : "off"}
      >
        {screen.at === "menu" ? (
          <Menu
            onPlay={(fresh) => {
              void game.refresh();
              setScreen(fresh ? { at: "intro", next: "desktop" } : { at: "desktop" });
            }}
            onIntro={() => setScreen({ at: "intro", next: "menu" })}
          />
        ) : (
          <Intro
            motion={prefs.motion}
            finalLabel={screen.next === "desktop" ? "Começar o primeiro dia" : "Voltar ao menu"}
            onDone={() => setScreen({ at: screen.next })}
          />
        )}
      </div>
    );
  }

  const busy = state?.job?.running ?? false;
  const setClock = (paused: boolean) => void game.act(() => api("POST", "/api/clock", { paused }));
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
            reading={state.reading}
            clock={state.clock}
            canPause={state.session?.phase === "calm" || state.session?.phase === "incident"}
            onClock={setClock}
            onInstalled={(name) =>
              setToast(`${name} está no topology.yaml. Faça um deploy para ligar.`)
            }
          />
        );
      case "missions":
        return (
          <Missions
            state={state}
            busy={busy}
            onAdvance={(canonical) =>
              void game.act(() => api("POST", "/api/advance", { canonical }))
            }
          />
        );
      case "git":
        return <Git hasSession={state.session !== null} jobVersion={state.job?.id ?? 0} />;
      case "lab":
        return (
          <Lab
            state={state}
            busy={busy}
            onRun={(p) => void game.act(() => api("POST", "/api/lab", p))}
          />
        );
      case "support":
        return <Support running={state.running} />;
      case "notes":
        return <Notes hasSession={state.session !== null} />;
      case "profile":
        return <Profile career={state.career} />;
      case "settings":
        return (
          <Settings
            prefs={prefs}
            setPrefs={setPrefs}
            state={state}
            busy={busy}
            onStop={(down) => void game.act(() => api("POST", "/api/stop", { down }))}
            onReplayTour={() => setTouring(true)}
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
        <button
          type="button"
          className="logo"
          onClick={() => setScreen({ at: "menu" })}
          title="Menu principal: saves e abertura"
        >
          <img src="/logo-mark.png" alt="" />
          LerdOS
        </button>
        <span className="topbar-title">{state?.scenario.title ?? ""}</span>
        <span className="topbar-status">
          {state?.session && (
            <span className={`pill ${state.session.phase}`}>{PHASE[state.session.phase]}</span>
          )}
          {state && (
            <span className={state.running ? "pill live" : "pill"}>
              {state.running ? "Loja no ar" : "Loja desligada"}
            </span>
          )}
          {state?.clock.paused && (
            <button
              type="button"
              className="pill paused"
              title="Nenhuma mensagem chega e nenhum prazo corre. A loja continua recebendo tráfego."
              onClick={() => setClock(false)}
            >
              ⏸ Relógio parado · retomar
            </button>
          )}
          {state?.deadline && (
            <Countdown deadline={state.deadline} clock={state.clock} onOpen={() => open("inbox")} />
          )}
          {state?.career && (
            <button
              type="button"
              className="pill career"
              onClick={() => open("profile")}
              title="Perfil e carreira"
            >
              {state.career.level} · {money(state.career.balance)}
            </button>
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

      {showTour ? (
        <Tour motion={prefs.motion} onDone={endTour} />
      ) : (
        <Slotthy state={state} busy={game.act} onOpen={open} motion={prefs.motion} />
      )}

      <nav className="dock" aria-label="Apps">
        {APPS.map((a) => (
          <button
            type="button"
            key={a.id}
            className="dock-item"
            data-app={a.id}
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
