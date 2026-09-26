import { createContext, useContext, useEffect, useState } from "react";
import { api, type CatalogEntry, type Change, type Size, type Topology } from "../api.ts";
import { ACTION, COMPONENT_NAME, usd, useTopology } from "./useTopology.ts";

type Prices = Record<string, Partial<Record<Size, number>>>;

// Per-instance prices, read once from the catalog (the server owns them).
const PricesContext = createContext<Prices>({});

function usePrices(): Prices {
  const [prices, setPrices] = useState<Prices>({});
  useEffect(() => {
    api<CatalogEntry[]>("GET", "/api/catalog").then(
      (cat) => {
        const out: Prices = {};
        for (const e of cat) {
          out[e.kind] = Object.fromEntries(Object.entries(e.sizes).map(([s, t]) => [s, t?.price]));
        }
        setPrices(out);
      },
      () => setPrices({}),
    );
  }, []);
  return prices;
}

function Box({
  id,
  size,
  detail,
  count = 1,
  change,
}: {
  id: string;
  size: Size;
  detail: string;
  count?: number;
  change?: Change;
}) {
  const price = useContext(PricesContext)[id]?.[size];
  return (
    <li className="arch-box" data-change={change?.action}>
      <div className="arch-head">
        <strong>{COMPONENT_NAME[id]}</strong>
        {change && <span className="arch-badge">{ACTION[change.action]}</span>}
      </div>
      <span className="sr-only">
        {count} {count === 1 ? "instância" : "instâncias"} de tamanho {size}
      </span>
      <div className="arch-replicas" aria-hidden="true">
        {Array.from({ length: count }, (_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: replicas are identical and ordered
          <span key={i} className="arch-replica">
            {size}
          </span>
        ))}
      </div>
      <div className="muted small">{detail}</div>
      {price !== undefined && (
        <div className="small">
          {usd(price * count)}
          <span className="muted">/mês</span>
        </div>
      )}
    </li>
  );
}

function Diagram({ t, pending }: { t: Topology; pending: Change[] }) {
  const c = t.components;
  const ch = (id: string) => pending.find((p) => p.component === id);
  return (
    <ol className="arch" aria-label="Caminho de uma requisição">
      <li className="arch-edge-start">Clientes</li>
      <Box
        id="nginx"
        size={c.nginx.size}
        detail="config: nginx/default.conf"
        change={ch("nginx")}
      />
      <Box
        id="app"
        size={c.app.size}
        count={c.app.replicas}
        detail={`pool de ${c.app.dbMaxConns} conexões por réplica → ${c.app.database}`}
        change={ch("app")}
      />
      {c.pgbouncer && (
        <Box
          id="pgbouncer"
          size={c.pgbouncer.size}
          detail={`${c.pgbouncer.poolMode}, ${c.pgbouncer.defaultPoolSize} conexões com o banco`}
          change={ch("pgbouncer")}
        />
      )}
      <Box
        id="postgres"
        size={c.postgres.size}
        detail={`max_connections ${c.postgres.maxConnections}`}
        change={ch("postgres")}
      />
    </ol>
  );
}

/** Arquitetura: what topology.yaml describes, what it costs, what a deploy changes. */
export function Architecture({
  hasSession,
  jobVersion,
  onOpen,
}: {
  hasSession: boolean;
  jobVersion: unknown;
  onOpen: (app: "deploy" | "docs") => void;
}) {
  const { view, error } = useTopology(hasSession, jobVersion);
  const prices = usePrices();
  if (!hasSession)
    return <p className="empty">A arquitetura aparece quando o primeiro dia começa.</p>;
  if (!view) return <p className="empty">{error ?? "Lendo o topology.yaml…"}</p>;

  const pending = view.pending ?? [];
  const removed = pending.filter((p) => p.action === "remove");
  const over = view.budget > 0 && view.workspace_cost > view.budget;
  return (
    <PricesContext value={prices}>
      <div className="architecture">
        <div className="arch-costs">
          <div>
            <span className="muted small">no workspace</span>
            <strong className={over ? "arch-cost-value error-text" : "arch-cost-value"}>
              {usd(view.workspace_cost)}/mês
            </strong>
          </div>
          <div>
            <span className="muted small">em produção</span>
            <strong className="arch-cost-value">
              {view.deployed ? `${usd(view.deployed_cost)}/mês` : "—"}
            </strong>
          </div>
          {view.budget > 0 && (
            <div>
              <span className="muted small">orçamento</span>
              <strong className="arch-cost-value">{usd(view.budget)}/mês</strong>
            </div>
          )}
        </div>
        {view.budget > 0 && (
          <div className="cpu-track budget" aria-hidden="true">
            <span
              className={over ? "cpu-fill hot" : "cpu-fill"}
              style={{ width: `${Math.min(100, (view.workspace_cost / view.budget) * 100)}%` }}
            />
          </div>
        )}

        {view.workspace_error ? (
          <div className="panel">
            <h3>topology.yaml inválido</h3>
            <pre className="joblog">{view.workspace_error}</pre>
          </div>
        ) : (
          view.workspace && <Diagram t={view.workspace} pending={pending} />
        )}
        {removed.length > 0 && (
          <p className="muted small">
            Sai no próximo deploy: {removed.map((r) => COMPONENT_NAME[r.component]).join(", ")}.
          </p>
        )}

        <div className="actions">
          {pending.length > 0 ? (
            <>
              <span>O workspace mudou em relação à produção.</span>
              <button type="button" className="btn" onClick={() => onOpen("deploy")}>
                Abrir o Deploy
              </button>
            </>
          ) : (
            <span className="muted">Produção roda exatamente o que está no workspace.</span>
          )}
          <button type="button" className="btn ghost" onClick={() => onOpen("docs")}>
            Catálogo de componentes
          </button>
        </div>
        <p className="muted small">
          O diagrama é o <code>topology.yaml</code> do seu workspace. Edite no seu editor; esta tela
          acompanha.
        </p>
      </div>
    </PricesContext>
  );
}
