import { useEffect, useState } from "react";
import { type GameState, type Message, viewed } from "../api.ts";
import { Markdown } from "../markdown.tsx";

const fmtTime = (iso: string) =>
  iso && !iso.startsWith("0001")
    ? new Date(iso).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" })
    : "";

export const messageKey = (m: Message) => `${m.id}@${m.at}`;

const WELCOME: Message = {
  id: "welcome",
  from: "RH do Mercado Lerdo",
  subject: "Seu primeiro dia",
  at: "",
  markdown: [
    "Oi! Este é o seu computador no **Mercado Lerdo**, o marketplace dos pequenos vendedores, que acabou de contratar a primeira pessoa de engenharia: você.",
    "",
    "Ao começar, a gente prepara o seu **workspace** (uma cópia do código do site, que você edita no seu editor) e liga a produção numa sandbox só sua.",
    "",
    "A primeira vez leva alguns minutos.",
  ].join("\n"),
};

export function Inbox({
  state,
  seen,
  onSeen,
  onStart,
  onNext,
  busy,
}: {
  state: GameState;
  seen: string[];
  onSeen: (key: string) => void;
  onStart: () => void;
  onNext: () => void;
  busy: boolean;
}) {
  const list = state.session ? [...state.messages].reverse() : [WELCOME];
  const newest = list[0]?.id ?? "";
  const [openId, setOpenId] = useState(newest);
  const [shownNewest, setShownNewest] = useState(newest);
  // A new mail (the incident, the debrief) opens by itself.
  if (newest !== shownNewest) {
    setShownNewest(newest);
    setOpenId(newest);
  }
  const current = list.find((m) => m.id === openId) ?? list[0];
  const currentKey = current && state.session ? messageKey(current) : null;
  useEffect(() => {
    if (currentKey) onSeen(currentKey);
  }, [currentKey, onSeen]);
  // Once per opening of the debrief, like the CLI prints it once per PASS.
  const debriefOpen = currentKey?.startsWith("debrief@") ?? false;
  useEffect(() => {
    if (debriefOpen) void viewed("debrief");
  }, [debriefOpen]);

  const starting = state.job?.running && state.job.kind === "play";
  return (
    <div className="inbox">
      <nav className="inbox-list" aria-label="Mensagens">
        {list.map((m) => {
          const unread = state.session && !seen.includes(messageKey(m));
          return (
            <button
              type="button"
              key={m.id}
              className="inbox-item"
              aria-current={m.id === current?.id ? "true" : undefined}
              onClick={() => setOpenId(m.id)}
            >
              <span className="inbox-from">
                {unread && <span className="dot" role="img" aria-label="não lida" />}
                {m.from}
                <time>{fmtTime(m.at)}</time>
              </span>
              <span className="inbox-subject">{m.subject}</span>
            </button>
          );
        })}
      </nav>
      <article className="inbox-read" key={current?.id}>
        {current && (
          <>
            <header className="inbox-head">
              <strong>{current.subject}</strong>
              <span className="muted">de {current.from}</span>
            </header>
            <Markdown source={current.markdown} />
            {starting && state.job && (
              <div className="joblog" role="log" aria-live="polite">
                {(state.job.lines ?? []).map((l) => (
                  <div key={l}>{l}</div>
                ))}
              </div>
            )}
            {!state.session && (
              <div className="actions">
                <button type="button" className="btn" disabled={busy} onClick={onStart}>
                  {starting ? "Preparando…" : "Começar o primeiro dia"}
                </button>
              </div>
            )}
            {state.session && !state.running && current.id === list[0]?.id && (
              <div className="actions">
                <p className="muted">A loja está desligada. O seu código continua no workspace.</p>
                <button type="button" className="btn" disabled={busy} onClick={onStart}>
                  {starting ? "Ligando…" : "Ligar a loja"}
                </button>
              </div>
            )}
            {current.id === "calm" && state.session?.phase === "calm" && state.running && (
              <div className="actions">
                <button type="button" className="btn" disabled={busy} onClick={onNext}>
                  Começar o expediente
                </button>
              </div>
            )}
          </>
        )}
      </article>
    </div>
  );
}
