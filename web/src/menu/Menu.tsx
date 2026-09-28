import { useCallback, useEffect, useState } from "react";
import { api, type Saves, type SaveView } from "../api.ts";
import { money } from "../money.ts";
import { lastPlayed, playedAgo } from "./saves.ts";

const TAGLINES = [
  "Sem pressa. Com quem faz.",
  "Carregando… devagarinho.",
  "Hoje: 130 visitantes, zero pressa.",
  "Feito à mão, com calma (e com índices).",
  "O nome é Lerdo. O site não pode ser.",
  "Nenhuma preguiça foi apressada na produção deste jogo.",
];

// What the sloth says when poked too much.
const POKES = ["", "", "Hm?", "Oi…", "Calma.", "Sem pressa!", "Eu tô trabalhando, tá?"];

const PHASE = { calm: "dia calmo", incident: "incidente", completed: "resolvido" } as const;

/**
 * Menu: the title screen. Continue the last game, start a new one (up to
 * three saves, ADR-0025), pick or delete a save, or watch the intro again.
 */
export function Menu({
  onPlay,
  onIntro,
}: {
  /** fresh is true for a save with no game yet: the intro plays first. */
  onPlay: (fresh: boolean) => void;
  onIntro: () => void;
}) {
  const [saves, setSaves] = useState<Saves | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [working, setWorking] = useState(false);
  const [confirm, setConfirm] = useState<number | null>(null);
  const [tag, setTag] = useState(0);
  const [pokes, setPokes] = useState(0);
  const [now, setNow] = useState(() => Date.now());

  const load = useCallback(() => {
    api<Saves>("GET", "/api/saves").then(setSaves, (e: unknown) =>
      setError(e instanceof Error ? e.message : String(e)),
    );
  }, []);
  useEffect(load, [load]);

  useEffect(() => {
    const id = setInterval(() => {
      setTag((t) => (t + 1) % TAGLINES.length);
      setNow(Date.now());
    }, 4200);
    return () => clearInterval(id);
  }, []);

  // A confirmation to delete lasts a few seconds.
  useEffect(() => {
    if (confirm === null) return;
    const id = setTimeout(() => setConfirm(null), 4000);
    return () => clearTimeout(id);
  }, [confirm]);

  const run = async (path: string, body: object | undefined, then: (s: Saves) => void) => {
    setWorking(true);
    setError(null);
    try {
      const s = await api<Saves>("POST", path, body);
      setSaves(s);
      then(s);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setWorking(false);
    }
  };

  const play = (s: SaveView) => run("/api/saves/select", { slot: s.slot }, () => onPlay(s.empty));
  const fresh = () => run("/api/saves/new", undefined, () => onPlay(true));
  const remove = (s: SaveView) => {
    if (confirm !== s.slot) {
      setConfirm(s.slot);
      return;
    }
    setConfirm(null);
    void run("/api/saves/delete", { slot: s.slot }, () => undefined);
  };

  const list = saves?.saves ?? [];
  const last = lastPlayed(list);
  const full = list.length > 0 && list.every((s) => !s.empty);

  // Enter continues the last game, or starts one.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Enter" || working || !saves || e.target instanceof HTMLButtonElement) return;
      if (last) void play(last);
      else if (!full) void fresh();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  return (
    <div className="menu">
      <div className="menu-leaves" aria-hidden="true">
        <span />
        <span />
        <span />
        <span />
        <span />
      </div>
      <header className="menu-title">
        <button
          type="button"
          className={pokes >= 3 ? "menu-logo poked" : "menu-logo"}
          onClick={() => setPokes((p) => Math.min(p + 1, POKES.length - 1))}
          aria-label="Mercado Lerdo"
        >
          <img src="/logo.png" alt="Mercado Lerdo" />
        </button>
        {POKES[pokes] && (
          <span className="menu-poke" role="status">
            {POKES[pokes]}
          </span>
        )}
        <p className="menu-tagline" key={tag}>
          {TAGLINES[tag]}
        </p>
      </header>

      <nav className="menu-main" aria-label="Menu principal">
        {last && (
          <button
            type="button"
            className="menu-btn primary"
            disabled={working}
            onClick={() => void play(last)}
          >
            Continuar
            <small>
              Save {last.slot} · {last.scenario}
            </small>
          </button>
        )}
        <button
          type="button"
          className={last ? "menu-btn" : "menu-btn primary"}
          disabled={working || full || !saves}
          onClick={() => void fresh()}
          title={full ? "Os três saves estão ocupados: apague um para começar outro." : undefined}
        >
          Nova partida
          <small>{full ? "os 3 saves estão ocupados" : "o primeiro dia de trabalho"}</small>
        </button>
        <button type="button" className="menu-btn ghost" disabled={working} onClick={onIntro}>
          Ver a abertura
        </button>
      </nav>

      <section className="menu-saves" aria-label="Saves">
        {list.map((s) => (
          <article key={s.slot} className={s.empty ? "save empty" : "save"}>
            <header>
              <strong>Save {s.slot}</strong>
              {s.current && !s.empty && <span className="save-tag">aberto</span>}
            </header>
            {s.empty ? (
              <p className="muted">Vazio. Uma nova história cabe aqui.</p>
            ) : (
              <>
                <p className="save-scenario">{s.scenario}</p>
                <p className="muted small">
                  {s.phase ? PHASE[s.phase] : ""} · {s.missions}{" "}
                  {s.missions === 1 ? "missão concluída" : "missões concluídas"}
                </p>
                <p className="muted small">
                  {s.level} · {money(s.balance)} · {playedAgo(s.played_at, now)}
                </p>
              </>
            )}
            <footer>
              <button
                type="button"
                className="btn small"
                disabled={working}
                onClick={() => void play(s)}
              >
                {s.empty ? "Começar aqui" : "Jogar"}
              </button>
              {!s.empty && (
                <button
                  type="button"
                  className={confirm === s.slot ? "btn small danger" : "btn small ghost"}
                  disabled={working}
                  onClick={() => remove(s)}
                >
                  {confirm === s.slot ? "Apagar mesmo?" : "Apagar"}
                </button>
              )}
            </footer>
          </article>
        ))}
      </section>

      {confirm !== null && (
        <p className="menu-warn" role="status">
          Apagar o Save {confirm} apaga o código que você escreveu e o progresso dele. Clique de
          novo para confirmar.
        </p>
      )}
      {working && (
        <p className="menu-warn">Trocando de save… (a loja do save anterior é desligada)</p>
      )}
      {error && (
        <p className="error-text" role="alert">
          {error}
        </p>
      )}
      <footer className="menu-foot">
        Um jogo sobre engenharia de software de verdade: a loja é de mentira, o sistema não.
      </footer>
    </div>
  );
}
