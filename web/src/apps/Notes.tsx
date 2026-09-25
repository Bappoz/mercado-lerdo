import { useEffect, useRef, useState } from "react";
import { api } from "../api.ts";

/** Notes: the player's scratchpad, saved in the workspace (.sesim/notes.md). */
export function Notes({ hasSession }: { hasSession: boolean }) {
  const [text, setText] = useState<string | null>(null);
  const [saved, setSaved] = useState<"saved" | "saving" | "error">("saved");
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);

  useEffect(() => {
    if (!hasSession) return;
    api<{ text: string }>("GET", "/api/notes")
      .then((r) => setText(r.text))
      .catch(() => setText(""));
  }, [hasSession]);
  useEffect(() => () => clearTimeout(timer.current), []);

  if (!hasSession)
    return (
      <p className="empty">
        As notas ficam no seu workspace, que nasce quando o primeiro dia começa.
      </p>
    );
  if (text === null) return <p className="empty">Abrindo…</p>;

  const onChange = (v: string) => {
    setText(v);
    setSaved("saving");
    clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      api("PUT", "/api/notes", { text: v })
        .then(() => setSaved("saved"))
        .catch(() => setSaved("error"));
    }, 700);
  };

  return (
    <div className="notes">
      <label htmlFor="notes-text" className="sr-only">
        Notas
      </label>
      <textarea
        id="notes-text"
        value={text}
        onChange={(e) => onChange(e.target.value)}
        placeholder="Hipóteses, números do dia calmo, o que você já tentou…"
        spellCheck={false}
      />
      <p className="muted small" aria-live="polite">
        {
          {
            saved: "salvo no workspace (.sesim/notes.md)",
            saving: "salvando…",
            error: "não consegui salvar",
          }[saved]
        }
      </p>
    </div>
  );
}
