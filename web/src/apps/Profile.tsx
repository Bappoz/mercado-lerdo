import { useEffect, useState } from "react";
import { api, type CareerView, type Profile as ProfileData } from "../api.ts";
import { money } from "../money.ts";
import { ICONS, Pixels } from "../os/pixel.tsx";

const AVATAR_LEGEND = { "#": "var(--icon-ink)", a: "var(--icon-accent)" };

const fmtDay = (at: string) =>
  new Date(at).toLocaleString("pt-BR", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });

/** Profile: the player's career at Mercado Lerdo (ADR-0023). */
export function Profile({ career }: { career: CareerView | null }) {
  const [p, setP] = useState<ProfileData | null>(null);
  const [error, setError] = useState<string | null>(null);
  // The glance in the game state changes whenever money or experience moves.
  const key = career ? `${career.balance}/${career.xp}` : "";
  useEffect(() => {
    if (!key) return;
    api<ProfileData>("GET", "/api/profile").then(setP, (e: unknown) =>
      setError(e instanceof Error ? e.message : String(e)),
    );
  }, [key]);

  if (!career) return <p className="empty">O seu perfil começa no primeiro dia.</p>;
  if (error) return <p className="empty">{error}</p>;
  const span = career.next_xp !== undefined ? career.next_xp - career.level_xp : 0;
  const pct =
    span > 0 ? Math.min(100, Math.round(((career.xp - career.level_xp) / span) * 100)) : 100;

  return (
    <div className="profile">
      <header className="profile-head">
        <Pixels bitmap={ICONS.profile} legend={AVATAR_LEGEND} scale={7} className="avatar" />
        <div>
          <p className="muted small">Engenharia · Mercado Lerdo</p>
          <h3>{career.level}</h3>
          <div
            className="xp-bar"
            role="progressbar"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={pct}
            aria-label="Experiência até o próximo nível"
          >
            <span style={{ width: `${pct}%` }} />
          </div>
          <p className="muted small">
            {career.next
              ? `${career.xp} de ${career.next_xp} de experiência para ${career.next}`
              : "Topo da carreira"}
          </p>
        </div>
      </header>

      <dl className="profile-cards">
        <div>
          <dt>Saldo</dt>
          <dd className={career.balance < 0 ? "neg" : undefined}>{money(career.balance)}</dd>
        </div>
        <div>
          <dt>Salário por missão</dt>
          <dd>{p ? money(p.level.salary) : "…"}</dd>
        </div>
        <div>
          <dt>Próxima dica</dt>
          <dd>{career.hint_price === 0 ? "grátis" : money(career.hint_price)}</dd>
        </div>
        <div>
          <dt>Próximo experimento no Lab</dt>
          <dd>{career.lab_price === 0 ? "grátis" : money(career.lab_price)}</dd>
        </div>
      </dl>
      {career.balance < 0 && (
        <p className="muted small">
          O Otávio adiantou o salário: o próximo holerite cobre a diferença.
        </p>
      )}

      {p && p.payslips.length > 0 && (
        <section>
          <h4>Holerites</h4>
          {[...p.payslips].reverse().map((s) => (
            <details key={s.mission} className="payslip">
              <summary>
                <strong>{s.title}</strong> · {s.level} · {money(s.total)} · +{s.xp} XP
              </summary>
              <table className="grid">
                <tbody>
                  {s.lines.map((l) => (
                    <tr key={l.label}>
                      <td>{l.label}</td>
                      <td className="num">{money(l.amount)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </details>
          ))}
        </section>
      )}

      {p && (
        <section>
          <h4>Extrato</h4>
          <table className="grid">
            <tbody>
              {p.statement.map((l) => (
                <tr key={`${l.at}-${l.kind}-${l.label}`}>
                  <td className="muted small">{fmtDay(l.at)}</td>
                  <td>{l.label}</td>
                  <td className={l.amount < 0 ? "num neg" : "num"}>{money(l.amount)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}

      <p className="muted small">
        O salário é seu, não da empresa: paga dicas extras da Tabby e experimentos a mais no Lab.
        Cursos e ferramentas de observabilidade chegam nos próximos stages.
      </p>
    </div>
  );
}
