import type { GameState } from "../api.ts";
import { ACTION, COMPONENT_NAME, usd, useTopology } from "./useTopology.ts";

/** Deploy: ship the workspace to production, keeping its data. */
export function Deploy({
  state,
  busy,
  onDeploy,
}: {
  state: GameState;
  busy: boolean;
  onDeploy: () => void;
}) {
  const job = state.job;
  const { view } = useTopology(state.session !== null, job?.id && !job.running ? job.id : 0);
  const running = job?.running ?? false;
  const invalid = view?.workspace_error ?? "";
  const pending = view?.pending ?? [];
  const last = job?.kind === "deploy" ? job : null;

  if (!state.session) return <p className="empty">Nada para publicar antes do primeiro dia.</p>;
  return (
    <div className="tests">
      <section className="panel">
        <header className="panel-head">
          <div>
            <h3>Deploy para produção</h3>
            <p className="muted">
              Leva tudo o que está no workspace: o código (recompila), as migrações novas, o{" "}
              <code>topology.yaml</code> e o <code>nginx/default.conf</code>. Os dados de produção
              ficam. Componentes recriados perdem as requisições em andamento.
            </p>
          </div>
          <button
            type="button"
            className="btn"
            disabled={busy || running || !state.running || invalid !== ""}
            onClick={onDeploy}
          >
            {running && job?.kind === "deploy" ? "Publicando…" : "Fazer deploy"}
          </button>
        </header>
        {invalid && <pre className="error-text">{invalid}</pre>}
        {!invalid && view && (
          <>
            <h3 className="panel-sub">Na infraestrutura</h3>
            {pending.length === 0 ? (
              <p className="muted">Nada muda: produção já roda este topology.yaml.</p>
            ) : (
              <ul className="changes">
                {pending.map((c) => (
                  <li key={c.component} data-change={c.action}>
                    <span className="arch-badge">{ACTION[c.action]}</span>{" "}
                    {COMPONENT_NAME[c.component]}
                  </li>
                ))}
              </ul>
            )}
            <p
              className={
                view.budget > 0 && view.workspace_cost > view.budget
                  ? "error-text small"
                  : "muted small"
              }
            >
              Custo depois do deploy: {usd(view.workspace_cost)}/mês
              {view.deployed ? ` (hoje ${usd(view.deployed_cost)})` : ""}
              {view.budget > 0 ? ` · orçamento ${usd(view.budget)}` : ""}
            </p>
          </>
        )}
        {!state.running && (
          <p className="muted">A Nexora está desligada. Ligue pela Caixa de entrada.</p>
        )}
      </section>

      {last && (
        <section className="panel">
          <h3>
            {last.running
              ? "Deploy em andamento"
              : last.error
                ? "O último deploy falhou"
                : "Último deploy"}
          </h3>
          <div className="joblog" role="log" aria-live="polite">
            {(last.lines ?? []).map((l, i) => (
              // biome-ignore lint/suspicious/noArrayIndexKey: an append-only log
              <div key={i}>{l}</div>
            ))}
          </div>
          {last.error && <pre className="error-text">{last.error}</pre>}
          {!last.running && !last.error && (
            <p className="muted small">
              Concluído às {new Date(last.finished ?? last.started).toLocaleTimeString("pt-BR")}.
            </p>
          )}
        </section>
      )}
    </div>
  );
}
