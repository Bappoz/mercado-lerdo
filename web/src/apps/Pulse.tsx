import { useEffect, useState } from "react";
import { api, type Pulse as PulseData, type Series, viewed } from "../api.ts";
import { Chart, fmtPct, fmtRate, fmtSeconds, Legend, last } from "./Chart.tsx";

const SPANS = [
  { s: 300, label: "5 min" },
  { s: 900, label: "15 min" },
  { s: 1800, label: "30 min" },
];

const byName = (a: Series, b: Series) => a.name.localeCompare(b.name);

/** Pulse: production right now, as the clients and the containers feel it. */
export function Pulse({ running }: { running: boolean }) {
  const [span, setSpan] = useState(900);
  const [data, setData] = useState<PulseData | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (running) void viewed("pulse");
  }, [running]);

  useEffect(() => {
    if (!running) return;
    let alive = true;
    const load = async () => {
      try {
        const d = await api<PulseData>("GET", `/api/pulse?span=${span}`);
        if (alive) {
          setData(d);
          setErr(null);
        }
      } catch (e) {
        if (alive) setErr(e instanceof Error ? e.message : String(e));
      }
    };
    void load();
    const id = setInterval(load, 5000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [running, span]);

  if (!running)
    return <p className="empty">A Nexora está desligada: não há produção para medir.</p>;
  if (!data) return <p className="empty">{err ?? "Lendo a produção…"}</p>;

  const p95 = [...data.p95].sort(byName);
  const rps = [...data.rps].sort(byName);
  const errs = [...data.errors].sort(byName);
  const cpu = [...data.cpu].sort(byName);
  const noTraffic = rps.every((s) => !s.points?.length);

  return (
    <div className="pulse">
      <div className="pulse-bar">
        <span className="muted">tráfego visto pelos clientes · atualiza a cada 5 s</span>
        <fieldset className="segmented">
          <legend className="sr-only">Janela de tempo</legend>
          {SPANS.map((o) => (
            <button
              type="button"
              key={o.s}
              aria-pressed={span === o.s}
              onClick={() => setSpan(o.s)}
            >
              {o.label}
            </button>
          ))}
        </fieldset>
      </div>
      {err && <p className="warn-text">Última leitura falhou: {err}</p>}
      {noTraffic && (
        <p className="muted">
          Ainda sem tráfego: ele começa alguns segundos depois de a Nexora subir.
        </p>
      )}

      <section className="panel">
        <h3>Latência p95 por endpoint</h3>
        <Chart
          series={p95}
          from={data.from}
          to={data.to}
          format={fmtSeconds}
          log
          floor={0.01}
          label="Latência p95 por endpoint"
        />
        <Legend series={p95} value={(s) => fmtSeconds(last(s))} />
      </section>

      <section className="panel">
        <h3>Requisições por segundo</h3>
        <Chart
          series={rps}
          from={data.from}
          to={data.to}
          format={(v) => v.toFixed(0)}
          floor={1}
          label="Requisições por segundo"
        />
        <Legend series={rps} value={(s) => fmtRate(last(s))} />
      </section>
      <section className="panel">
        <h3>Erros (fração das requisições)</h3>
        <Chart
          series={errs}
          from={data.from}
          to={data.to}
          format={fmtPct}
          floor={0.05}
          label="Taxa de erros por endpoint"
        />
        <Legend series={errs} value={(s) => fmtPct(last(s))} />
      </section>

      <section className="panel">
        <h3>CPU de cada container em relação ao limite dele</h3>
        {cpu.length === 0 ? (
          <p className="muted">Sem leitura de CPU ainda.</p>
        ) : (
          <ul className="cpu">
            {cpu.map((s) => {
              const v = last(s) ?? 0;
              return (
                <li key={s.name}>
                  <span className="cpu-name">{s.name}</span>
                  <span className="cpu-track">
                    <span
                      className={v >= 0.85 ? "cpu-fill hot" : "cpu-fill"}
                      style={{ width: `${Math.min(100, v * 100)}%` }}
                    />
                  </span>
                  <span className="cpu-val">{fmtPct(v)}</span>
                </li>
              );
            })}
          </ul>
        )}
      </section>
    </div>
  );
}
