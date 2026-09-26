import type {
  CheckResult,
  GameState,
  Job,
  ObjectiveResult,
  Report,
  Signals,
  Verdict,
} from "../api.ts";
import { fmtPct, fmtRate, fmtSeconds } from "./Chart.tsx";

const VERDICT: Record<Verdict, string> = {
  PASS: "passou",
  FAIL: "reprovou",
  INCONCLUSIVE: "inconclusivo",
};

const DIMENSION: Record<string, string> = {
  correctness: "Corretude",
  latency: "Latência",
  availability: "Disponibilidade",
  resources: "Recursos",
};

const seconds = (ns: number) => `${Math.round(ns / 1e9)} s`;

function JobLog({ job }: { job: Job }) {
  return (
    <div className="joblog" role="log" aria-live="polite">
      {(job.lines ?? []).slice(-12).map((l, i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: an append-only log
        <div key={i}>{l}</div>
      ))}
      {job.running && <div className="blink">▍</div>}
    </div>
  );
}

function SignalsTable({ now, before }: { now: Signals; before?: Signals }) {
  const prev = (name: string) => before?.endpoints?.find((e) => e.name === name);
  return (
    <>
      <table className="grid">
        <thead>
          <tr>
            <th>endpoint</th>
            <th>req/s</th>
            <th>erros</th>
            <th>p95</th>
            {before && <th>p95 antes</th>}
          </tr>
        </thead>
        <tbody>
          {(now.endpoints ?? []).map((e) => (
            <tr key={e.name}>
              <td>{e.name}</td>
              <td>{fmtRate(e.rps)}</td>
              <td>{fmtPct(e.error_rate)}</td>
              <td>{e.rps > 0 ? fmtSeconds(e.p95_s) : "—"}</td>
              {before && <td>{fmtSeconds(prev(e.name)?.p95_s)}</td>}
            </tr>
          ))}
        </tbody>
      </table>
      <ul className="cpu compact">
        {Object.entries(now.cpu)
          .sort()
          .map(([c, v]) => (
            <li key={c}>
              <span className="cpu-name">{c}</span>
              <span className="cpu-track">
                <span
                  className={v >= 0.85 ? "cpu-fill hot" : "cpu-fill"}
                  style={{ width: `${Math.min(100, v * 100)}%` }}
                />
              </span>
              <span className="cpu-val">{fmtPct(v)}</span>
            </li>
          ))}
      </ul>
    </>
  );
}

function fmtMeasure(o: ObjectiveResult): string {
  if (o.measure.endsWith("_ms")) return `${o.median.toFixed(o.median < 10 ? 1 : 0)} ms`;
  if (o.measure.includes("ratio") || o.measure.includes("rate") || o.measure.includes("cpu"))
    return fmtPct(o.median);
  return String(o.median);
}

function ReportView({ report }: { report: Report }) {
  const dims = Object.entries(report.dimensions).sort();
  return (
    <>
      <ul className="verdicts">
        {dims.map(([d, v]) => (
          <li key={d} className={`verdict ${v.toLowerCase()}`}>
            <span>{DIMENSION[d] ?? d}</span>
            <strong>{VERDICT[v]}</strong>
          </li>
        ))}
      </ul>
      {report.host?.slow && (
        <p className="warn-text">
          Esta máquina rodou {report.host.slowdown.toFixed(1)}x mais devagar do que a máquina em que
          os cenários foram calibrados (teste de velocidade do banco:{" "}
          {Math.round(report.host.db_probe_ms)} ms; limite: {Math.round(report.host.limit_ms)} ms).
          Um resultado perto do limite ficou inconclusivo; o que reprova mesmo descontando a
          lentidão continua reprovado. Ligue o carregador ou saia do modo de economia de energia e
          teste de novo.
        </p>
      )}
      <table className="grid">
        <thead>
          <tr>
            <th>o que foi medido</th>
            <th>regra</th>
            <th>mediana</th>
            <th>veredito</th>
          </tr>
        </thead>
        <tbody>
          {(report.objectives ?? []).map((o) => (
            <tr key={o.id}>
              <td>
                <code>{o.measure}</code>
              </td>
              <td>
                <code>{o.rule}</code>
              </td>
              <td>{fmtMeasure(o)}</td>
              <td>
                {VERDICT[o.verdict]}
                {o.reason && <div className="muted small">{o.reason}</div>}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}

/** Tests: the Quick Check (indicative) and the Production Test (graded). */
export function Tests({
  state,
  busy,
  onCheck,
  onTest,
}: {
  state: GameState;
  busy: boolean;
  onCheck: () => void;
  onTest: () => void;
}) {
  const job = state.job;
  const jobBusy = job?.running ?? false;
  const disabled = busy || jobBusy || !state.running || !state.session;
  const check: CheckResult | undefined = job?.kind === "check" ? job.check : undefined;
  const lastSignals = state.session?.last_check;

  return (
    <div className="tests">
      <section className="panel">
        <header className="panel-head">
          <div>
            <h3>Quick Check</h3>
            <p className="muted">
              Recompila o seu workspace, aplica as migrações e mede 30 s do tráfego de agora. Rápido
              e indicativo: sem reset de dados.
            </p>
          </div>
          <button type="button" className="btn" disabled={disabled} onClick={onCheck}>
            {jobBusy && job?.kind === "check" ? "Medindo…" : "Rodar Quick Check"}
          </button>
        </header>
        {job?.kind === "check" && (job.running || job.error) && <JobLog job={job} />}
        {job?.kind === "check" && job.error && <p className="error-text">{job.error}</p>}
        {check ? (
          <>
            <p className="muted small">
              últimos {check.now.window_s} s · {seconds(check.elapsed)} no total
            </p>
            <SignalsTable now={check.now} before={check.before} />
          </>
        ) : (
          lastSignals &&
          !jobBusy && (
            <>
              <p className="muted small">último Quick Check</p>
              <SignalsTable now={lastSignals} />
            </>
          )
        )}
      </section>

      <section className="panel">
        <header className="panel-head">
          <div>
            <h3>Production Test</h3>
            <p className="muted">
              O que vale. Cada execução recria o banco original, aplica as suas migrações e roda o
              pico de tráfego. O tráfego do dia pausa enquanto isso; leva alguns minutos.
            </p>
          </div>
          <button type="button" className="btn" disabled={disabled} onClick={onTest}>
            {jobBusy && job?.kind === "test" ? "Testando…" : "Rodar Production Test"}
          </button>
        </header>
        {job?.kind === "test" && (job.running || job.error) && <JobLog job={job} />}
        {job?.kind === "test" && job.error && <p className="error-text">{job.error}</p>}
        {state.last_test && !(jobBusy && job?.kind === "test") && (
          <>
            <p className="muted small">
              {job?.kind === "test" && job.test ? `${seconds(job.test.elapsed)} no total · ` : ""}
              teste de {new Date(state.last_test.finished).toLocaleTimeString("pt-BR")}
            </p>
            <ReportView report={state.last_test} />
          </>
        )}
      </section>
      {!state.running && state.session && (
        <p className="muted">A Nexora está desligada. Ligue de novo pela Caixa de entrada.</p>
      )}
    </div>
  );
}
