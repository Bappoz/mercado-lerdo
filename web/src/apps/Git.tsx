import { useCallback, useEffect, useState } from "react";
import { api, type Commit, type FileChange } from "../api.ts";

const STATUS: Record<FileChange["status"], string> = {
  modified: "M",
  added: "A",
  deleted: "D",
  untracked: "novo",
  renamed: "R",
};

/** Git: the workspace's changes, their diff, history and commits. */
export function Git({ hasSession, jobVersion }: { hasSession: boolean; jobVersion: number }) {
  const [changes, setChanges] = useState<FileChange[]>([]);
  const [log, setLog] = useState<Commit[]>([]);
  const [path, setPath] = useState("");
  const [sinceStart, setSinceStart] = useState(false);
  const [diff, setDiff] = useState("");
  const [msg, setMsg] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [s, l] = await Promise.all([
        api<{ changes: FileChange[] }>("GET", "/api/git/status"),
        api<{ commits: Commit[] }>("GET", "/api/git/log"),
      ]);
      setChanges(s.changes);
      setLog(l.commits);
      setErr(null);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  }, []);

  // biome-ignore lint/correctness/useExhaustiveDependencies: jobVersion only triggers a reload
  useEffect(() => {
    if (!hasSession) return;
    void load();
    const id = setInterval(() => void load(), 5000);
    return () => clearInterval(id);
  }, [hasSession, load, jobVersion]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: changes only triggers a reload
  useEffect(() => {
    if (!hasSession) return;
    const q = new URLSearchParams({ path, since: sinceStart ? "start" : "" });
    api<{ diff: string }>("GET", `/api/git/diff?${q}`).then(
      (r) => setDiff(r.diff),
      (e: unknown) => setDiff(e instanceof Error ? e.message : String(e)),
    );
  }, [hasSession, path, sinceStart, changes]);

  if (!hasSession)
    return <p className="empty">O repositório nasce com o workspace, no primeiro dia.</p>;

  const commit = async () => {
    try {
      const c = await api<Commit>("POST", "/api/git/commit", { message: msg });
      setNote(`Commit ${c.short} criado.`);
      setMsg("");
      await load();
    } catch (e) {
      setNote(e instanceof Error ? e.message : String(e));
    }
  };

  return (
    <div className="git">
      <aside className="git-side">
        <h3>Mudanças ({changes.length})</h3>
        <fieldset className="segmented">
          <legend className="sr-only">Comparar com</legend>
          <button type="button" aria-pressed={!sinceStart} onClick={() => setSinceStart(false)}>
            último commit
          </button>
          <button type="button" aria-pressed={sinceStart} onClick={() => setSinceStart(true)}>
            início do cenário
          </button>
        </fieldset>
        <ul className="git-files">
          <li>
            <button type="button" aria-pressed={path === ""} onClick={() => setPath("")}>
              todas
            </button>
          </li>
          {changes.map((c) => (
            <li key={c.path}>
              <button type="button" aria-pressed={path === c.path} onClick={() => setPath(c.path)}>
                <span className="git-status" data-status={c.status}>
                  {STATUS[c.status]}
                </span>{" "}
                {c.path}
              </button>
            </li>
          ))}
        </ul>
        <form
          className="git-commit"
          onSubmit={(e) => {
            e.preventDefault();
            void commit();
          }}
        >
          <label htmlFor="git-msg">Mensagem do commit</label>
          <textarea
            id="git-msg"
            value={msg}
            onChange={(e) => setMsg(e.target.value)}
            placeholder="O que mudou e por quê"
            rows={3}
          />
          <button
            type="submit"
            className="btn small"
            disabled={!msg.trim() || changes.length === 0}
          >
            Commitar tudo
          </button>
          {note && <p className="muted small">{note}</p>}
        </form>
        <h3>Histórico</h3>
        <ol className="git-log">
          {log.map((c) => (
            <li key={c.hash}>
              <code>{c.short}</code> {c.subject}
              {c.refs?.map((r) => (
                <span key={r} className="arch-badge">
                  {r.replace("sesim/scenario-start/", "início: ")}
                </span>
              ))}
            </li>
          ))}
        </ol>
      </aside>
      <section className="git-diff" aria-label="Diff">
        {err && <p className="error-text">{err}</p>}
        {diff === "" ? (
          <p className="empty">
            Nada mudou{sinceStart ? " desde o início do cenário" : " desde o último commit"}.
          </p>
        ) : (
          <pre className="diff">
            {diff.split("\n").map((line, i) => (
              // biome-ignore lint/suspicious/noArrayIndexKey: lines of one diff, never reordered
              <span key={i} className={diffClass(line)}>
                {line}
                {"\n"}
              </span>
            ))}
          </pre>
        )}
      </section>
    </div>
  );
}

function diffClass(line: string): string | undefined {
  if (line.startsWith("+++") || line.startsWith("---") || line.startsWith("diff ")) return "d-file";
  if (line.startsWith("@@")) return "d-hunk";
  if (line.startsWith("+")) return "d-add";
  if (line.startsWith("-")) return "d-del";
  return undefined;
}
