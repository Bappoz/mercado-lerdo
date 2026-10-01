import { useState } from "react";
import { api, type GameState, type Hint, type PastStep, type TutorialView } from "../api.ts";
import { Markdown } from "../markdown.tsx";
import { money } from "../money.ts";
import { tabbyMood, tabbyTip } from "../tabby.ts";
import type { AppId } from "../theme.ts";
import { TabbySprite } from "./pixel.tsx";
import { useAnimate, useIdlePose } from "./TabbyActor.tsx";

type View =
  | { kind: "tip" }
  | { kind: "hint"; hint: Hint }
  | { kind: "history" }
  | { kind: "past"; at: number } // a step of the guided mode that is behind, by its place in `past`
  | { kind: "none" };

/**
 * Tabby, the mascot: scripted lines about what is going on, and the
 * scenario's hint ladder, one rung at a time and only when asked.
 */
export function Tabby({
  state,
  busy,
  onOpen,
  motion,
}: {
  state: GameState | null;
  busy: <T>(f: () => Promise<T>) => Promise<T | undefined>;
  onOpen: (app: AppId) => void;
  motion: boolean;
}) {
  const tip = tabbyTip(state);
  const [view, setView] = useState<View>({ kind: "tip" });
  const [tipId, setTipId] = useState(tip.id);
  const animate = useAnimate(motion);
  // A new situation reopens the bubble (derived during render, not in an effect).
  if (tip.id !== tipId) {
    setTipId(tip.id);
    setView({ kind: "tip" });
  }

  // The bubble stays while the player works in the windows (what she says is
  // often something to type there): only Tabby herself, or "Ok", puts it away.

  // She talks whenever a new line appears in her bubble.
  const talkKey =
    view.kind === "tip"
      ? `tip-${tip.id}`
      : view.kind === "hint"
        ? `hint-${view.hint.level}`
        : view.kind === "past"
          ? `past-${view.at}`
          : view.kind;
  const pose = useIdlePose(tabbyMood(state), animate, view.kind === "none" ? "" : talkKey);

  const hints = state?.hints ?? [];
  const total = state?.hints_total ?? 0;
  // The ladder is about the incident: nothing to hint at before it starts.
  const canHint = state?.session?.phase === "incident" && hints.length < total;
  const askHint = async () => {
    const r = await busy(() => api<{ hint: Hint | null }>("POST", "/api/hint"));
    if (r?.hint) setView({ kind: "hint", hint: r.hint });
  };
  const tutorial = (body: { action: "ok" | "help" | "answer"; choice?: string }) =>
    void busy(() => api("POST", "/api/tutorial", body));
  // The steps of the guided mode already behind: one may have closed before
  // the player read it (the migration written early), so she tells it again.
  const past = state?.tutorial?.past ?? [];
  const reviewing = view.kind === "past" ? past[view.at] : undefined;
  // Another game has other steps: back to what she says now.
  if (view.kind === "past" && !reviewing) setView({ kind: "tip" });

  return (
    <aside className="tabby" aria-label="Tabby, a assistente">
      {view.kind !== "none" && (
        <div className="bubble" role="status">
          {view.kind === "tip" &&
            (tip.tutorial ? <TutorialStep t={tip.tutorial} onAct={tutorial} /> : <p>{tip.text}</p>)}
          {view.kind === "hint" && (
            <>
              <p className="bubble-kind">
                Dica {view.hint.level} de {total} · {view.hint.kind}
              </p>
              <Markdown source={view.hint.text} />
            </>
          )}
          {reviewing && <PastStepView step={reviewing} total={state?.tutorial?.total ?? 0} />}
          {view.kind === "history" && (
            <ol className="hint-history">
              {hints.map((h) => (
                <li key={h.level}>
                  <p className="bubble-kind">{h.kind}</p>
                  <Markdown source={h.text} />
                </li>
              ))}
            </ol>
          )}
          <div className="bubble-actions">
            {view.kind === "past" && (
              <>
                <button
                  type="button"
                  className="btn small ghost"
                  disabled={view.at === 0}
                  onClick={() => setView({ kind: "past", at: view.at - 1 })}
                >
                  ‹ Anterior
                </button>
                {view.at < past.length - 1 ? (
                  <button
                    type="button"
                    className="btn small ghost"
                    onClick={() => setView({ kind: "past", at: view.at + 1 })}
                  >
                    Próximo ›
                  </button>
                ) : (
                  <button
                    type="button"
                    className="btn small"
                    onClick={() => setView({ kind: "tip" })}
                  >
                    Voltar para agora
                  </button>
                )}
              </>
            )}
            {view.kind !== "past" && past.length > 0 && (
              <button
                type="button"
                className="btn small ghost"
                onClick={() => setView({ kind: "past", at: past.length - 1 })}
              >
                ‹ Passo anterior
              </button>
            )}
            {view.kind === "tip" && tip.tutorial?.continue && (
              <button
                type="button"
                className="btn small"
                onClick={() => tutorial({ action: "ok" })}
              >
                Continuar
              </button>
            )}
            {view.kind === "tip" && tip.tutorial?.can_help && (
              <button
                type="button"
                className="btn small ghost"
                onClick={() => tutorial({ action: "help" })}
              >
                Me ajuda
              </button>
            )}
            {canHint && !tip.tutorial && (
              <button type="button" className="btn small" onClick={() => void askHint()}>
                {hints.length === 0 ? "Pedir uma dica" : "Próxima dica"}
                {state?.career &&
                  ` (${state.career.hint_price === 0 ? "grátis" : money(state.career.hint_price)})`}
              </button>
            )}
            {hints.length > 0 && view.kind !== "history" && (
              <button
                type="button"
                className="btn small ghost"
                onClick={() => setView({ kind: "history" })}
              >
                Dicas anteriores ({hints.length})
              </button>
            )}
            {!state?.session && (
              <button type="button" className="btn small ghost" onClick={() => onOpen("inbox")}>
                Abrir e-mail
              </button>
            )}
            {state?.session?.phase === "calm" && state.reading.length > 0 && (
              <button type="button" className="btn small ghost" onClick={() => onOpen("docs")}>
                Leituras da missão ({state.reading.length})
              </button>
            )}
            <button
              type="button"
              className="btn small ghost"
              onClick={() => setView({ kind: "none" })}
            >
              Ok
            </button>
          </div>
          {canHint && !tip.tutorial && hints.length === 0 && (
            <p className="muted small">
              A primeira dica de cada missão é grátis; as seguintes saem do seu salário. Sem saldo,
              o Otávio adianta.
            </p>
          )}
        </div>
      )}
      <button
        type="button"
        className="tabby-sprite"
        aria-label={view.kind === "none" ? "Falar com a Tabby" : "Esconder a Tabby"}
        onClick={() => setView(view.kind === "none" ? { kind: "tip" } : { kind: "none" })}
      >
        <TabbySprite pose={pose} />
      </button>
    </aside>
  );
}

/** A step of the guided mode that is behind, told again: nothing to answer. */
function PastStepView({ step, total }: { step: PastStep; total: number }) {
  return (
    <div className="tutorial-step">
      <p className="bubble-kind">
        Treinamento · passo {step.index} de {total} · {step.skipped ? "pulado" : "revendo"}
      </p>
      <Markdown source={step.text} />
      {step.done && (
        <div className="tutorial-before">
          <Markdown source={step.done} />
        </div>
      )}
    </div>
  );
}

/** One step of the guided mode (ADR-0024): what Tabby says, and the answers she offers. */
function TutorialStep({
  t,
  onAct,
}: {
  t: TutorialView;
  onAct: (body: { action: "answer"; choice: string }) => void;
}) {
  return (
    <div className="tutorial-step">
      <p className="bubble-kind">
        Treinamento · passo {t.index} de {t.total}
        {t.kind === "pergunta" && " · pense antes"}
      </p>
      {t.before && (
        <div className="tutorial-before">
          <Markdown source={t.before} />
        </div>
      )}
      <Markdown source={t.text} />
      {t.choices && (
        <div className="tutorial-choices">
          {t.choices.map((c) => (
            <button
              key={c.id}
              type="button"
              className="btn small ghost"
              onClick={() => onAct({ action: "answer", choice: c.id })}
            >
              {c.id}) {c.text}
            </button>
          ))}
        </div>
      )}
      {t.wrong && (
        <div className="warn-text">
          <Markdown source={t.wrong} />
        </div>
      )}
      {t.help && (
        <div className="tutorial-help">
          <Markdown source={t.help} />
        </div>
      )}
    </div>
  );
}
