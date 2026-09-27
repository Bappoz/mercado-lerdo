import { useEffect, useRef, useState } from "react";
import { api, type GameState, type Hint } from "../api.ts";
import { Markdown } from "../markdown.tsx";
import { tabbyMood, tabbyTip } from "../tabby.ts";
import type { AppId } from "../theme.ts";
import { TabbySprite } from "./pixel.tsx";
import { useAnimate, useIdlePose } from "./TabbyActor.tsx";

type View = { kind: "tip" } | { kind: "hint"; hint: Hint } | { kind: "history" } | { kind: "none" };

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
  const self = useRef<HTMLElement>(null);
  const animate = useAnimate(motion);
  // A new situation reopens the bubble (derived during render, not in an effect).
  if (tip.id !== tipId) {
    setTipId(tip.id);
    setView({ kind: "tip" });
  }

  // The bubble floats over the windows: any click elsewhere puts it away, so
  // it never swallows a click meant for the app under it.
  const showing = view.kind !== "none";
  useEffect(() => {
    if (!showing) return;
    const onDown = (e: PointerEvent) => {
      if (!self.current?.contains(e.target as Node)) setView({ kind: "none" });
    };
    document.addEventListener("pointerdown", onDown, true);
    return () => document.removeEventListener("pointerdown", onDown, true);
  }, [showing]);

  // She talks whenever a new line appears in her bubble.
  const talkKey =
    view.kind === "tip"
      ? `tip-${tip.id}`
      : view.kind === "hint"
        ? `hint-${view.hint.level}`
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

  return (
    <aside className="tabby" aria-label="Tabby, a assistente" ref={self}>
      {view.kind !== "none" && (
        <div className="bubble" role="status">
          {view.kind === "tip" && <p>{tip.text}</p>}
          {view.kind === "hint" && (
            <>
              <p className="bubble-kind">
                Dica {view.hint.level} de {total} · {view.hint.kind}
              </p>
              <Markdown source={view.hint.text} />
            </>
          )}
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
            {canHint && (
              <button type="button" className="btn small" onClick={() => void askHint()}>
                {hints.length === 0 ? "Pedir uma dica" : "Próxima dica"}
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
            <button
              type="button"
              className="btn small ghost"
              onClick={() => setView({ kind: "none" })}
            >
              Ok
            </button>
          </div>
          {canHint && hints.length === 0 && (
            <p className="muted small">
              Dicas contam no debrief, mas não tiram pontos. Use quando travar.
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
