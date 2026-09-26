// Client of the sesim UI server (internal/ui). The session token arrives in
// the URL fragment (#t=…), which never leaves the browser, and is kept in
// sessionStorage so a reload keeps working (threat model T1).

const TOKEN_KEY = "sesim.token";

function readToken(): string {
  const fromHash = new URLSearchParams(location.hash.slice(1)).get("t");
  if (fromHash) {
    try {
      sessionStorage.setItem(TOKEN_KEY, fromHash);
    } catch {
      // storage blocked: the token still works until the tab is reloaded
    }
    history.replaceState(null, "", location.pathname + location.search);
    return fromHash;
  }
  try {
    return sessionStorage.getItem(TOKEN_KEY) ?? "";
  } catch {
    return "";
  }
}

export const token = readToken();

// A new `sesim ui` run opened in this same tab changes only the fragment,
// which does not reload the page: reload so the new token is read.
window.addEventListener("hashchange", () => {
  if (new URLSearchParams(location.hash.slice(1)).has("t")) location.reload();
});

export class ApiError extends Error {
  readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  let data: unknown;
  try {
    data = text ? JSON.parse(text) : undefined;
  } catch {
    data = undefined;
  }
  if (!res.ok) {
    const msg = (data as { error?: string } | undefined)?.error ?? (text || res.statusText);
    throw new ApiError(res.status, msg);
  }
  return data as T;
}

/** viewed records that the player looked at something (product events, as the CLI records them). */
export function viewed(what: "debrief" | "pulse"): Promise<void> {
  return api<void>("POST", "/api/viewed", { what }).catch(() => undefined);
}

// ---- server types (internal/ui, internal/game, internal/play, internal/evaluator) ----

export type Phase = "calm" | "incident" | "completed";

export interface Endpoint {
  name: string;
  rps: number;
  error_rate: number;
  p95_s: number;
}

export interface Signals {
  window_s: number;
  endpoints: Endpoint[] | null;
  cpu: Record<string, number>;
}

export interface Session {
  phase: Phase;
  workspace: string;
  started_at: string;
  incident_at?: string;
  done_at?: string;
  hints_shown: number;
  checks: number;
  tests: number;
  last_check?: Signals;
}

export interface Message {
  id: string;
  from: string;
  subject: string;
  at: string;
  markdown: string;
}

export interface Hint {
  level: number;
  kind: string;
  text: string;
}

export type Verdict = "PASS" | "FAIL" | "INCONCLUSIVE";

export interface ObjectiveResult {
  id: string;
  dimension: string;
  measure: string;
  rule: string;
  values: number[] | null;
  median: number;
  verdict: Verdict;
  reason?: string;
}

export interface Report {
  scenario: string;
  started: string;
  finished: string;
  objectives: ObjectiveResult[] | null;
  dimensions: Record<string, Verdict>;
}

export interface CheckResult {
  now: Signals;
  before?: Signals;
  elapsed: number; // nanoseconds
}

export interface TestResult {
  report: Report;
  passed: boolean;
  elapsed: number;
}

export type JobKind = "play" | "check" | "test" | "deploy";

export interface Job {
  id: number;
  kind: JobKind;
  running: boolean;
  lines: string[] | null;
  error?: string;
  started: string;
  finished?: string;
  check?: CheckResult;
  test?: TestResult;
  deploy?: Change[];
}

// ---- topology (internal/topology, internal/stack) ----

export type Size = "S" | "M" | "L";

export interface Topology {
  apiVersion: string;
  kind: string;
  components: {
    nginx: { size: Size };
    app: { size: Size; replicas: number; database: "postgres" | "pgbouncer"; dbMaxConns: number };
    postgres: { size: Size; maxConnections: number };
    pgbouncer?: { size: Size; poolMode: string; defaultPoolSize: number; maxClientConn: number };
  };
}

export interface Change {
  component: string;
  action: "create" | "recreate" | "remove";
}

export interface TopologyView {
  workspace: Topology | null;
  workspace_error: string;
  workspace_cost: number;
  deployed: Topology | null;
  deployed_cost: number;
  pending: Change[] | null;
  budget: number;
}

export interface Tier {
  cpus: number;
  memory: number;
  price: number;
}

export interface CatalogEntry {
  kind: string;
  name: string;
  optional: boolean;
  installed: boolean;
  summary: string;
  when_to_use: string;
  tradeoffs: string;
  settings: { field: string; range: string; doc: string }[];
  sizes: Partial<Record<Size, Tier>>;
  snippet: string;
}

export interface GameState {
  scenario: { id: string; title: string; version: string };
  session: Session | null;
  running: boolean;
  messages: Message[];
  hints: Hint[];
  hints_total: number;
  job: Job | null;
  last_test: Report | null;
}

export interface Series {
  name: string;
  points: [number, number][] | null;
}

export interface Pulse {
  from: number;
  to: number;
  step_s: number;
  p95: Series[];
  rps: Series[];
  errors: Series[];
  cpu: Series[];
}
