import { useEffect, useState } from "react";
import { api, type CampaignView, type GameState } from "../api.ts";

const STATE: Record<string, string> = {
  done: "concluída",
  current: "em andamento",
  locked: "bloqueada",
};

const minutes = (a: string, b: string) =>
  Math.max(0, Math.round((new Date(b).getTime() - new Date(a).getTime()) / 60000));

/** Missões: the stage's scenarios, what was done, and the way forward. */
export function Missions({
  state,
  busy,
  onAdvance,
}: {
  state: GameState;
  busy: boolean;
  onAdvance: (canonical: boolean) => void;
}) {
  const [view, setView] = useState<CampaignView | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const phase = state.session?.phase;
  const jobId = state.job?.running ? -1 : (state.job?.id ?? 0);
  // biome-ignore lint/correctness/useExhaustiveDependencies: reload when the phase or a job changes
  useEffect(() => {
    if (!state.session) return;
    api<CampaignView>("GET", "/api/campaign").then(setView, (e: unknown) =>
      setErr(e instanceof Error ? e.message : String(e)),
    );
  }, [phase, jobId, state.scenario.id]);

  if (!state.session)
    return <p className="empty">As missões aparecem quando o primeiro dia começa.</p>;
  if (!view) return <p className="empty">{err ?? "Abrindo o quadro de missões…"}</p>;
  if (view.missions.length === 0)
    return <p className="empty">Este cenário não faz parte de uma campanha.</p>;

  const next = view.missions.find((m) => m.state === "locked");
  return (
    <div className="tests">
      <h3 className="missions-stage">{view.stage}</h3>
      <ol className="missions">
        {view.missions.map((m, i) => (
          <li key={m.id} className="panel mission" data-state={m.state}>
            <div className="mission-head">
              <span className="mission-n">{i + 1}</span>
              <strong>{m.title}</strong>
              <span className="arch-badge">{STATE[m.state]}</span>
            </div>
            {m.result && (
              <p className="muted small">
                {minutes(m.result.started_at, m.result.done_at)} min · {m.result.tests} Production
                Test(s) · {m.result.checks} Quick Check(s) · {m.result.hints} dica(s)
                {m.result.canonical ? " · começou do estado canônico" : ""}
              </p>
            )}
          </li>
        ))}
      </ol>
      {view.can_advance && next && (
        <section className="panel">
          <h3>Próxima missão: {next.title}</h3>
          <p className="muted">
            O seu código segue com você: o próximo cenário só aplica a mudança da história (por
            exemplo, "subimos instâncias") ao seu <code>topology.yaml</code>. Se preferir, comece do
            estado canônico: o workspace vira a solução de referência, e o seu código fica no
            histórico do Git. A loja atual é desligada.
          </p>
          <div className="actions">
            <button type="button" className="btn" disabled={busy} onClick={() => onAdvance(false)}>
              Seguir com o meu código
            </button>
            <button
              type="button"
              className="btn ghost"
              disabled={busy}
              onClick={() => onAdvance(true)}
            >
              Começar do estado canônico
            </button>
          </div>
        </section>
      )}
      {!view.can_advance && view.missions.every((m) => m.state === "done") && (
        <section className="panel">
          <h3>Stage concluído</h3>
          <p>
            Fim da garagem: os três incidentes do Mercado Lerdo estão resolvidos. Obrigado por jogar
            esta versão.
          </p>
        </section>
      )}
    </div>
  );
}
