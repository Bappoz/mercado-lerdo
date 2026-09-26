import { useState } from "react";
import type { GameState, LabParams } from "../api.ts";
import { fmtPct, fmtSeconds } from "./Chart.tsx";

const PROFILES: { v: string; label: string }[] = [
  { v: "normal-day", label: "Dia normal (navegação, pouco checkout)" },
  { v: "flash-sale", label: "Flash sale (busca pesada)" },
  { v: "rush", label: "Rush (25 % checkout)" },
  { v: "crash-day", label: "Compras com clientes que tentam de novo" },
];

/** Laboratório de carga: try the system under load before the test that counts. */
export function Lab({
  state,
  busy,
  onRun,
}: {
  state: GameState;
  busy: boolean;
  onRun: (p: LabParams) => void;
}) {
  const [p, setP] = useState<LabParams>({ profile: "rush", rate: 40, duration: 40, crash: false });
  const job = state.job?.kind === "lab" ? state.job : null;
  const r = job && !job.running ? job.lab : undefined;
  const disabled = busy || (state.job?.running ?? false) || !state.running || !state.session;

  return (
    <div className="tests">
      <section className="panel">
        <h3>Novo experimento</h3>
        <p className="muted small">
          Roda carga contra a produção como ela está agora, sem apagar dados e sem nota. O tráfego
          do dia pausa enquanto isso. Faça um deploy antes, se mudou algo.
        </p>
        <form
          className="lab-form"
          onSubmit={(e) => {
            e.preventDefault();
            onRun(p);
          }}
        >
          <label>
            Perfil
            <select value={p.profile} onChange={(e) => setP({ ...p, profile: e.target.value })}>
              {PROFILES.map((o) => (
                <option key={o.v} value={o.v}>
                  {o.label}
                </option>
              ))}
            </select>
          </label>
          <label>
            Taxa no pico (req/s)
            <input
              type="number"
              min={1}
              max={150}
              value={p.rate}
              onChange={(e) => setP({ ...p, rate: Number(e.target.value) })}
            />
          </label>
          <label>
            Duração no pico (s)
            <input
              type="number"
              min={20}
              max={120}
              value={p.duration}
              onChange={(e) => setP({ ...p, duration: Number(e.target.value) })}
            />
          </label>
          <label className="check">
            <input
              type="checkbox"
              checked={p.crash}
              onChange={(e) => setP({ ...p, crash: e.target.checked })}
            />
            Derrubar a instância 1 do app no meio
          </label>
          <button type="submit" className="btn" disabled={disabled}>
            {job?.running ? "Rodando…" : "Rodar experimento"}
          </button>
        </form>
      </section>

      {job && (job.running || job.error) && (
        <section className="panel">
          <div className="joblog" role="log" aria-live="polite">
            {(job.lines ?? []).map((l, i) => (
              // biome-ignore lint/suspicious/noArrayIndexKey: an append-only log
              <div key={i}>{l}</div>
            ))}
          </div>
          {job.error && <pre className="error-text">{job.error}</pre>}
        </section>
      )}

      {r && (
        <section className="panel">
          <h3>Resultado</h3>
          <p className="muted small">
            {r.params.profile} até {r.params.rate} req/s por {r.params.duration} s
            {r.crashed ? " · instância 1 derrubada no meio" : ""} · erros no total:{" "}
            <strong>{fmtPct(r.error_rate)}</strong>
          </p>
          <table className="grid">
            <thead>
              <tr>
                <th>página</th>
                <th>requisições</th>
                <th>erros</th>
                <th>p50</th>
                <th>p95</th>
                <th>p99</th>
              </tr>
            </thead>
            <tbody>
              {r.endpoints.map((e) => (
                <tr key={e.name}>
                  <td>{e.name}</td>
                  <td>{e.requests}</td>
                  <td>{fmtPct(e.error_rate)}</td>
                  <td>{fmtSeconds(e.p50_ms / 1000)}</td>
                  <td>{fmtSeconds(e.p95_ms / 1000)}</td>
                  <td>{fmtSeconds(e.p99_ms / 1000)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {r.payments && (
            <p className={Object.values(r.payments).some((v) => v > 0) ? "error-text" : "muted"}>
              Cobranças × pedidos: {r.payments.lost_orders} cobrados sem pedido,{" "}
              {r.payments.duplicate_charges} cobrados em dobro, {r.payments.duplicate_orders}{" "}
              pedidos duplicados, {r.payments.phantom_orders} pagos sem cobrança.
            </p>
          )}
        </section>
      )}
    </div>
  );
}
