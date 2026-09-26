import { useEffect, useState } from "react";
import { api, type CatalogEntry, type Size } from "../api.ts";
import { Markdown } from "../markdown.tsx";
import { usd } from "./useTopology.ts";

const SIZES: Size[] = ["S", "M", "L"];

/** Docs: the component catalog; optional components install with one click. */
export function Docs({
  hasSession,
  onInstalled,
}: {
  hasSession: boolean;
  onInstalled: (name: string) => void;
}) {
  const [entries, setEntries] = useState<CatalogEntry[] | null>(null);
  const [open, setOpen] = useState("nginx");
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
      <nav className="inbox-list" aria-label="Componentes">
        {entries.map((x) => (
          <button
            type="button"
            key={x.kind}
            className="inbox-item"
            aria-current={x.kind === e.kind ? "true" : undefined}
            onClick={() => setOpen(x.kind)}
          >
            <span className="inbox-from">{x.name}</span>
            <span className="inbox-subject">
              {x.optional ? (x.installed ? "instalado" : "opcional") : "sempre presente"}
            </span>
          </button>
        ))}
      </nav>
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
    </div>
  );
}
