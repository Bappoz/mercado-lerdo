import { useEffect, useState } from "react";
import { api, type Ticket, viewed } from "../api.ts";

const KIND: Record<Ticket["kind"], string> = {
  lost_payment: "dinheiro",
  double_charge: "dinheiro",
  error: "erro",
  slow: "lentidão",
};

/** Suporte: what customers complained about in the last ten minutes. */
export function Support({ running }: { running: boolean }) {
  const [tickets, setTickets] = useState<Ticket[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (running) void viewed("support");
  }, [running]);

  useEffect(() => {
    if (!running) return;
    let alive = true;
    const load = async () => {
      try {
        const r = await api<{ tickets: Ticket[] }>("GET", "/api/tickets");
        if (alive) {
          setTickets(r.tickets);
          setErr(null);
        }
      } catch (e) {
        if (alive) setErr(e instanceof Error ? e.message : String(e));
      }
    };
    void load();
    const id = setInterval(load, 10_000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [running]);

  if (!running) return <p className="empty">A Nexora está desligada: ninguém consegue reclamar.</p>;
  if (!tickets) return <p className="empty">{err ?? "Abrindo a caixa do suporte…"}</p>;
  return (
    <div className="support">
      <p className="muted small">últimos 10 minutos · atualiza a cada 10 s</p>
      {err && <p className="warn-text">Última leitura falhou: {err}</p>}
      {tickets.length === 0 ? (
        <p className="empty">Nenhuma reclamação. Os clientes estão conseguindo comprar.</p>
      ) : (
        <ul className="tickets">
          {tickets.map((t) => (
            <li key={t.id} className="ticket" data-kind={t.kind}>
              <header className="ticket-head">
                <strong>{t.subject}</strong>
                <span className="arch-badge">{KIND[t.kind]}</span>
              </header>
              <blockquote>{t.quote}</blockquote>
              <p className="muted small">
                {t.kind === "slow"
                  ? "vários clientes"
                  : `${t.count} ${t.kind === "error" ? "requisições com erro" : t.count === 1 ? "pedido" : "pedidos"}`}
              </p>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
