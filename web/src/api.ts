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
export function viewed(what: "debrief" | "pulse" | "support"): Promise<void> {
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
  /** A story event's kind (ADR-0022); absent for the scenario's own mails. */
  kind?: "pedido" | "bronca" | "emergencia" | "elogio" | "noticia" | "reply";
}

/** The most urgent open request of the story. */
export interface Deadline {
  id: string;
  kind: string;
  title: string;
  from: string;
  goal: "test_pass" | "store_up" | "deploy" | "view_pulse" | "view_support";
  until: string;
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
  /** Present when the sandbox's database speed was probed (ADR-0021). */
  host?: {
    db_probe_ms: number;
    limit_ms: number;
    reference_ms: number;
    slowdown: number;
    slow: boolean;
    excused: boolean;
  };
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

export type JobKind = "play" | "check" | "test" | "deploy" | "lab" | "advance";

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
  lab?: LabResult;
}

export interface LabParams {
  profile: string;
  rate: number;
  duration: number;
  crash: boolean;
}

export interface LabResult {
  params: LabParams;
  endpoints: {
    name: string;
    requests: number;
    error_rate: number;
    p50_ms: number;
    p95_ms: number;
    p99_ms: number;
  }[];
  error_rate: number;
  crashed: boolean;
  payments?: {
    lost_orders: number;
    phantom_orders: number;
    duplicate_charges: number;
    duplicate_orders: number;
  };
  elapsed: number;
}

export interface Finished {
  id: string;
  title: string;
  started_at: string;
  done_at: string;
  checks: number;
  tests: number;
  hints: number;
  canonical?: boolean;
}

export interface Mission {
  dir: string;
  id: string;
  title: string;
  state: "done" | "current" | "locked";
  result?: Finished;
}

export interface CampaignView {
  stage?: string;
  missions: Mission[];
  can_advance?: boolean;
}

export interface FileChange {
  path: string;
  status: "modified" | "added" | "deleted" | "untracked" | "renamed";
}

export interface Commit {
  hash: string;
  short: string;
  author: string;
  date: string;
  subject: string;
  refs?: string[];
}

export interface Ticket {
  id: string;
  kind: "error" | "slow" | "lost_payment" | "double_charge";
  endpoint?: string;
  count: number;
  subject: string;
  quote: string;
  since: string;
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
  deadline: Deadline | null;
  career: CareerView | null;
  tutorial: TutorialView | null;
  reading: Reading[];
  clock: ClockView;
}

/** A piece of documentation Tabby recommends for the mission (ADR-0026). */
export interface Reading {
  id: string;
  title: string;
  source: string;
  url: string;
  lang: "pt" | "en";
  minutes: number;
  why: string;
}

/** The story's clock: the player may stop it to read (ADR-0026). */
export interface ClockView {
  paused: boolean;
  since?: string; // when it stopped
}

/** Tabby's guided mode (ADR-0024): the step she waits on. */
export interface TutorialView {
  step: string; // "" once every step is behind
  index: number;
  total: number;
  kind: "guia" | "pergunta" | "";
  before?: string; // what she says about the step just done
  text: string;
  help?: string; // once asked for
  can_help: boolean;
  choices?: { id: string; text: string }[];
  wrong?: string; // after a wrong answer
  continue: boolean;
  done: boolean;
  past?: PastStep[]; // the steps behind, oldest first: the player can go back to them
}

/** A step of the guided mode that is behind, as Tabby said it. */
export interface PastStep {
  step: string;
  index: number;
  kind: "guia" | "pergunta";
  text: string;
  done?: string; // what she said when it was done
  skipped?: boolean; // a calm step the incident overtook
}

/** The career at a glance (ADR-0023); money in whole reais. */
export interface CareerView {
  level: string;
  next?: string;
  xp: number;
  level_xp: number;
  next_xp?: number;
  balance: number;
  hint_price: number;
  lab_price: number;
}

export interface Line {
  at: string;
  kind: "hiring" | "salary" | "bonus" | "purchase";
  label: string;
  amount: number;
}

export interface Payslip {
  mission: string;
  title: string;
  level: string;
  lines: Line[];
  total: number;
  xp: number;
}

export interface Profile {
  level: { title: string; xp: number; salary: number };
  next?: { title: string; xp: number; salary: number };
  xp: number;
  balance: number;
  statement: Line[];
  payslips: Payslip[];
  promotions: { at: string; from: string; to: string; salary: number }[];
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

/** One slot of the save menu (ADR-0025). */
export interface SaveView {
  slot: number;
  current: boolean;
  empty: boolean;
  scenario?: string;
  phase?: Phase;
  missions: number;
  level?: string;
  balance: number;
  played_at?: string;
}

export interface Saves {
  saves: SaveView[];
  max: number;
}
