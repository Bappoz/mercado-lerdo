import { useEffect, useState } from "react";
import {
  api,
  type CatalogEntry,
  type ClockView,
  type Reading as ReadingEntry,
  type Size,
} from "../api.ts";
import { Markdown } from "../markdown.tsx";
import { usd } from "./useTopology.ts";

const SIZES: Size[] = ["S", "M", "L"];

const LANG: Record<ReadingEntry["lang"], string> = { pt: "em português", en: "em inglês" };

/**
 * Docs: what Tabby recommends reading for the mission (ADR-0026), and the
 * component catalog; optional components install with one click.
 */
export function Docs({
  hasSession,
  reading,
  clock,
  canPause,
  onClock,
  onInstalled,
}: {
  hasSession: boolean;
  reading: ReadingEntry[];
  clock: ClockView;
  canPause: boolean; // a mission is on: there is a clock to stop
  onClock: (paused: boolean) => void;
  onInstalled: (name: string) => void;
}) {
  const [entries, setEntries] = useState<CatalogEntry[] | null>(null);
  // A reading is "read:<id>"; anything else is a component's kind.
  const [open, setOpen] = useState(reading.length > 0 ? `read:${reading[0].id}` : "nginx");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = () =>
    api<CatalogEntry[]>("GET", "/api/catalog")
      .then(setEntries)
      .catch((e: unknown) => setErr(e instanceof Error ? e.message : String(e)));
  // biome-ignore lint/correctness/useExhaustiveDependencies: load once per session change
  useEffect(() => {
    void load();
  }, [hasSession]);

  if (!entries) return <p className="empty">{err ?? "Abrindo a documentação…"}</p>;
  const read = reading.find((r) => `read:${r.id}` === open);
  const e = entries.find((x) => x.kind === open) ?? entries[0];

  const install = async () => {
    setBusy(true);
    setErr(null);
    try {
      await api("POST", "/api/install", { kind: e.kind });
      await load();
      onInstalled(e.name);
    } catch (x) {
      setErr(x instanceof Error ? x.message : String(x));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="docs">
      <nav className="inbox-list" aria-label="Leituras e componentes">
        {reading.length > 0 && <p className="docs-group">Leituras da missão</p>}
        {reading.map((r) => (
          <button
            type="button"
            key={r.id}
            className="inbox-item"
            aria-current={r === read ? "true" : undefined}
            onClick={() => setOpen(`read:${r.id}`)}
          >
            <span className="inbox-from">{r.title}</span>
            <span className="inbox-subject">
              {r.source} · ≈ {r.minutes} min
            </span>
          </button>
        ))}
        <p className="docs-group">Componentes</p>
        {entries.map((x) => (
          <button
            type="button"
            key={x.kind}
            className="inbox-item"
            aria-current={!read && x.kind === e.kind ? "true" : undefined}
            onClick={() => setOpen(x.kind)}
          >
            <span className="inbox-from">{x.name}</span>
            <span className="inbox-subject">
              {x.optional ? (x.installed ? "instalado" : "opcional") : "sempre presente"}
            </span>
          </button>
        ))}
      </nav>
      {read ? (
        <ReadingPage key={read.id} r={read} clock={clock} canPause={canPause} onClock={onClock} />
      ) : (
        <article className="inbox-read" key={e.kind}>
          <header className="inbox-head">
            <strong>{e.name}</strong>
            <span className="muted">
              <code>components.{e.kind}</code> no topology.yaml
            </span>
          </header>
          <div className="md">
            <p>{e.summary}</p>
            <h3>Quando usar</h3>
            <Markdown source={e.when_to_use} />
            <h3>Custos e cuidados</h3>
            <Markdown source={e.tradeoffs} />
            <h3>Configuração</h3>
            <div className="md-table">
              <table>
                <thead>
                  <tr>
                    <th>campo</th>
                    <th>valores</th>
                    <th>o que faz</th>
                  </tr>
                </thead>
                <tbody>
                  {e.settings.map((s) => (
                    <tr key={s.field}>
                      <td>
                        <code>{s.field}</code>
                      </td>
                      <td>{s.range}</td>
                      <td>{s.doc}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <h3>Tamanhos</h3>
            <div className="md-table">
              <table>
                <thead>
                  <tr>
                    <th>tamanho</th>
                    <th>CPU</th>
                    <th>memória</th>
                    <th>custo</th>
                  </tr>
                </thead>
                <tbody>
                  {SIZES.filter((s) => e.sizes[s]).map((s) => {
                    const t = e.sizes[s];
                    return t ? (
                      <tr key={s}>
                        <td>{s}</td>
                        <td>{t.cpus}</td>
                        <td>{t.memory / (1 << 20)} MiB</td>
                        <td>{usd(t.price)}/mês por instância</td>
                      </tr>
                    ) : null;
                  })}
                </tbody>
              </table>
            </div>
          </div>
          {e.optional && (
            <div className="panel install">
              {e.installed ? (
                <p>
                  Já está no seu <code>topology.yaml</code>. Para tirar, apague o bloco{" "}
                  <code>{e.kind}</code> e faça um deploy.
                </p>
              ) : (
                <>
                  <p>
                    Instalar adiciona este bloco ao seu <code>topology.yaml</code>. Nada muda em
                    produção até o próximo deploy.
                  </p>
                  <pre className="joblog">{e.snippet}</pre>
                  <button
                    type="button"
                    className="btn"
                    disabled={busy || !hasSession}
                    onClick={() => void install()}
                  >
                    Instalar {e.name}
                  </button>
                </>
              )}
            </div>
          )}
          {err && <p className="error-text">{err}</p>}
        </article>
      )}
    </div>
  );
}

/** One recommended reading: why it is worth it, the link, and the story's clock. */
function ReadingPage({
  r,
  clock,
  canPause,
  onClock,
}: {
  r: ReadingEntry;
  clock: ClockView;
  canPause: boolean;
  onClock: (paused: boolean) => void;
}) {
  const since = clock.since
    ? new Date(clock.since).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" })
    : "";
  return (
    <article className="inbox-read">
      <header className="inbox-head">
        <strong>{r.title}</strong>
        <span className="muted">
          {r.source} · ≈ {r.minutes} min · {LANG[r.lang]}
        </span>
      </header>
      <div className="md">
        <h3>Por que a Tabby recomenda</h3>
        <Markdown source={r.why} />
      </div>
      <div className="actions">
        <a
          className="btn"
          href={r.url}
          target="_blank"
          rel="noopener noreferrer"
          // A product event: losing one must never get in the reader's way.
          onClick={() => void api("POST", "/api/reading", { id: r.id }).catch(() => undefined)}
        >
          Abrir a documentação ↗
        </a>
      </div>
      <p className="muted small">
        Abre em outra aba do navegador. Nenhuma leitura é obrigatória: elas explicam o assunto, não
        dão a resposta da missão.
      </p>
      {canPause && (
        <div className="panel reading-clock">
          {clock.paused ? (
            <>
              <p>
                <strong>⏸ O relógio da história está parado{since && ` desde ${since}`}.</strong>{" "}
                Nenhuma mensagem chega e nenhum prazo corre. A loja continua recebendo tráfego: só a
                história espera.
              </p>
              <button type="button" className="btn small" onClick={() => onClock(false)}>
                Retomar o relógio
              </button>
            </>
          ) : (
            <>
              <p>
                Vai ler com calma? Pare o relógio da história: as mensagens e os prazos esperam você
                voltar. Um deploy, um check ou um teste retomam o relógio sozinhos.
              </p>
              <button type="button" className="btn small ghost" onClick={() => onClock(true)}>
                Parar o relógio enquanto leio
              </button>
            </>
          )}
        </div>
      )}
    </article>
  );
}
