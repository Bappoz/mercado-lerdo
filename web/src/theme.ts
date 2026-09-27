// Themes are token sets (ADR-0016): every component reads CSS variables, never
// a theme by name. Personalization overrides tokens on top of the theme.

export type ThemeId = "nexora98" | "plantao" | "pastel";
export type Wallpaper = "cena" | "liso" | "grade";
export type Coat = "laranja" | "cinza" | "frajola";
export type TextSize = "P" | "M" | "G";
export type AppId =
  | "missions"
  | "inbox"
  | "pulse"
  | "terminal"
  | "tests"
  | "architecture"
  | "deploy"
  | "git"
  | "lab"
  | "support"
  | "docs"
  | "notes"
  | "settings";

export interface Accent {
  name: string;
  hex: string; // fill
  on: string; // text on the fill (≥ 4.5:1)
  dark: string; // pressed/shadow; also the accent as text on the window background
}

/** Tabby's palette: outline, fur, dark fur, white, eyes, pink. */
export type TabbyColors = Record<"k" | "o" | "d" | "w" | "e" | "p", string>;

export interface Theme {
  id: ThemeId;
  name: string;
  blurb: string;
  fonts: { chrome: string; body: string; mono: string };
  scene: "none" | "night" | "pastel";
  vars: Record<string, string>; // CSS custom properties, without the leading --
  titleBar: (accent: Accent, app: AppId) => { bg: string; ink: string };
  accents: Accent[];
  tabby: TabbyColors;
}

const nexora98: Theme = {
  id: "nexora98",
  name: "Lerdo 98",
  blurb: "quente, anos 90",
  fonts: {
    chrome: "'Pixelify Sans', 'Courier New', monospace",
    body: "'IBM Plex Sans', 'Segoe UI', sans-serif",
    mono: "'JetBrains Mono', 'Courier New', monospace",
  },
  scene: "none",
  vars: {
    desk: "#2f6d68",
    "desk-pattern": "repeating-conic-gradient(#357a74 0% 25%, transparent 0% 50%)",
    "desk-pattern-size": "4px 4px",
    grid: "#3a817a",
    ink: "#2a2320",
    muted: "#6b5d52",
    danger: "#b3261e",
    ok: "#256b42",
    warn: "#8a5a00",
    line: "#2a2320",
    bw: "2px",
    radius: "0px",
    "radius-sm": "0px",
    "bar-bg": "#f3ead7",
    "bar-ink": "#2a2320",
    "win-bg": "#f3ead7",
    "win-shadow": "inset 2px 2px 0 #fffaf0, inset -2px -2px 0 #d8cbb0, 6px 6px 0 #1c2f2d",
    "tbar-size": "17px",
    "tbar-case": "none",
    "tbar-spacing": "0.5px",
    panel: "#fffaf0",
    "btn-shadow": "inset -2px -2px 0 var(--accent-dark)",
    "chip-bg": "#fffaf0",
    "term-bg": "#1f1a17",
    "term-ink": "#f3ead7",
    "term-accent": "#f2b134",
    "term-dim": "#a89a8c",
    "chart-bg": "#fffaf0",
    "chart-grid": "#cbbd9f",
    "chart-area": "#f5c8b8",
    s1: "#b3261e",
    s2: "#1f5f8b",
    s3: "#256b42",
    s4: "#8a5a00",
    "cpu-track": "#e2d6bd",
    "cpu-hot": "#b3261e",
    "cpu-ok": "#256b42",
    "icon-ink": "#2a2320",
    "dock-bg": "#f3ead7",
    "dock-shadow": "inset 2px 2px 0 #fffaf0, 6px 6px 0 #1c2f2d",
    "dock-radius": "0px",
    "bubble-bg": "#fffaf0",
    "bubble-border": "#2a2320",
    "bubble-shadow": "6px 6px 0 #1c2f2d",
    "toast-bg": "#2a2320",
    "toast-ink": "#f3ead7",
  },
  titleBar: (a) => ({ bg: a.hex, ink: a.on }),
  accents: [
    { name: "Tijolo", hex: "#b8432b", on: "#fff8ec", dark: "#8c3220" },
    { name: "Petróleo", hex: "#1f5f8b", on: "#fff8ec", dark: "#174766" },
    { name: "Musgo", hex: "#3f6b2a", on: "#fff8ec", dark: "#2f501f" },
    { name: "Ameixa", hex: "#7a3b69", on: "#fff8ec", dark: "#5b2c4f" },
  ],
  tabby: { k: "#2a1d17", o: "#e8984a", d: "#a95a22", w: "#fff3e3", e: "#2a1d17", p: "#e98093" },
};

const plantao: Theme = {
  id: "plantao",
  name: "Plantão noturno",
  blurb: "madrugada de plantão",
  fonts: {
    chrome: "'Silkscreen', 'Courier New', monospace",
    body: "'Atkinson Hyperlegible', 'Segoe UI', sans-serif",
    mono: "'JetBrains Mono', 'Courier New', monospace",
  },
  scene: "night",
  vars: {
    desk: "#12152e",
    "desk-pattern": "radial-gradient(circle at 78% 18%, #1d2250 0, #12152e 55%)",
    "desk-pattern-size": "auto",
    grid: "#1e2350",
    ink: "#e8e6f5",
    muted: "#a6a9cf",
    danger: "#ff7a6b",
    ok: "#7dd3a8",
    warn: "#ffb454",
    line: "#3a4280",
    bw: "2px",
    radius: "8px",
    "radius-sm": "4px",
    "bar-bg": "#0c0f24",
    "bar-ink": "#e8e6f5",
    "win-bg": "#1f2448",
    "win-shadow": "0 0 0 2px #0b0e22, 8px 8px 0 #0b0e22",
    "tbar-size": "13px",
    "tbar-case": "uppercase",
    "tbar-spacing": "1px",
    panel: "#181c3b",
    "btn-shadow": "none",
    "chip-bg": "#181c3b",
    "term-bg": "#0b0d20",
    "term-ink": "#cfe3ff",
    "term-accent": "#7dd3a8",
    "term-dim": "#7f86b8",
    "chart-bg": "#181c3b",
    "chart-grid": "#3a4280",
    "chart-area": "#4a2f4f",
    s1: "#ff7a6b",
    s2: "#8fb8ff",
    s3: "#7dd3a8",
    s4: "#ffb454",
    "cpu-track": "#2d3470",
    "cpu-hot": "#ff7a6b",
    "cpu-ok": "#7dd3a8",
    "icon-ink": "#cfd2f5",
    "dock-bg": "#0c0f24",
    "dock-shadow": "0 0 0 2px #0b0e22, 0 10px 0 #070918",
    "dock-radius": "10px",
    "bubble-bg": "#1f2448",
    "bubble-border": "var(--accent)",
    "bubble-shadow": "8px 8px 0 #0b0e22",
    "toast-bg": "var(--accent)",
    "toast-ink": "var(--on-accent)",
  },
  titleBar: () => ({ bg: "#2b3160", ink: "#e8e6f5" }),
  accents: [
    { name: "Âmbar", hex: "#ffb454", on: "#12152e", dark: "#ffb454" },
    { name: "Menta", hex: "#7dd3a8", on: "#12152e", dark: "#7dd3a8" },
    { name: "Coral", hex: "#ff8a7a", on: "#12152e", dark: "#ff8a7a" },
    { name: "Lavanda", hex: "#b8a9ff", on: "#12152e", dark: "#b8a9ff" },
  ],
  tabby: { k: "#0b0d20", o: "#9aa3c9", d: "#5b6391", w: "#e8e6f5", e: "#ffb454", p: "#ff9aa9" },
};

const pastelBars: Record<AppId, string> = {
  inbox: "#f6c6a8",
  pulse: "#b9d7b0",
  terminal: "#f5e19a",
  tests: "#d7c7f0",
  architecture: "#b9d7b0",
  deploy: "#f6c6a8",
  support: "#f2a2ac",
  lab: "#b9d7b0",
  missions: "#f5e19a",
  git: "#d7c7f0",
  docs: "#cfe6f0",
  notes: "#f5e19a",
  settings: "#cfe6f0",
};

const pastel: Theme = {
  id: "pastel",
  name: "Estúdio pastel",
  blurb: "cozy e claro",
  fonts: {
    chrome: "'DotGothic16', 'Courier New', monospace",
    body: "'Nunito', 'Segoe UI', sans-serif",
    mono: "'JetBrains Mono', 'Courier New', monospace",
  },
  scene: "pastel",
  vars: {
    desk: "#bfe3ef",
    "desk-pattern": "none",
    "desk-pattern-size": "auto",
    grid: "#aed8e6",
    ink: "#3b3530",
    muted: "#6e645c",
    danger: "#b8364a",
    ok: "#2f7a4c",
    warn: "#94620f",
    line: "#3b3530",
    bw: "3px",
    radius: "14px",
    "radius-sm": "10px",
    "bar-bg": "#fffaf2",
    "bar-ink": "#3b3530",
    "win-bg": "#fffaf2",
    "win-shadow": "0 7px 0 #3b3530",
    "tbar-size": "17px",
    "tbar-case": "none",
    "tbar-spacing": "0",
    panel: "#fff3e2",
    "btn-shadow": "0 3px 0 #3b3530",
    "chip-bg": "#fff3e2",
    "term-bg": "#2d2a32",
    "term-ink": "#fdf6e3",
    "term-accent": "#f6b89a",
    "term-dim": "#b2a9b8",
    "chart-bg": "#fff3e2",
    "chart-grid": "#e0cdb0",
    "chart-area": "#f8cdd3",
    s1: "#b8364a",
    s2: "#3f6fa8",
    s3: "#2f7a4c",
    s4: "#94620f",
    "cpu-track": "#efe3cf",
    "cpu-hot": "#d9566a",
    "cpu-ok": "#6fae7f",
    "icon-ink": "#3b3530",
    "dock-bg": "#fffaf2",
    "dock-shadow": "0 7px 0 #3b3530",
    "dock-radius": "18px",
    "bubble-bg": "#fffaf2",
    "bubble-border": "#3b3530",
    "bubble-shadow": "0 7px 0 #3b3530",
    "toast-bg": "#3b3530",
    "toast-ink": "#fffaf2",
  },
  titleBar: (_, app) => ({ bg: pastelBars[app], ink: "#3b3530" }),
  accents: [
    { name: "Manteiga", hex: "#f5e19a", on: "#3b3530", dark: "#a6760a" },
    { name: "Pêssego", hex: "#f6c6a8", on: "#3b3530", dark: "#c96a3f" },
    { name: "Sálvia", hex: "#b9d7b0", on: "#3b3530", dark: "#4f8a5c" },
    { name: "Lilás", hex: "#d7c7f0", on: "#3b3530", dark: "#7a5fb0" },
  ],
  tabby: { k: "#3b3530", o: "#f0a35e", d: "#c9743a", w: "#fffaf2", e: "#3b3530", p: "#f28a9b" },
};

export const THEMES: Theme[] = [nexora98, plantao, pastel];

export interface Prefs {
  theme: ThemeId;
  wallpaper: Wallpaper;
  accent: number;
  chrome: "pixel" | "suave";
  text: TextSize;
  motion: boolean;
  coat: Coat;
  seen: string[]; // inbox messages already read
}

export const DEFAULT_PREFS: Prefs = {
  theme: "nexora98",
  wallpaper: "cena",
  accent: 0,
  chrome: "pixel",
  text: "M",
  motion: true,
  coat: "laranja",
  seen: [],
};

function oneOf<T>(v: unknown, allowed: readonly T[], fallback: T): T {
  return allowed.includes(v as T) ? (v as T) : fallback;
}

/** normalizePrefs accepts anything the server stored and keeps what it recognizes. */
export function normalizePrefs(raw: unknown): Prefs {
  const r = (raw && typeof raw === "object" ? raw : {}) as Record<string, unknown>;
  const d = DEFAULT_PREFS;
  return {
    theme: oneOf(r.theme, ["nexora98", "plantao", "pastel"] as const, d.theme),
    wallpaper: oneOf(r.wallpaper, ["cena", "liso", "grade"] as const, d.wallpaper),
    accent: oneOf(r.accent, [0, 1, 2, 3] as const, d.accent),
    chrome: oneOf(r.chrome, ["pixel", "suave"] as const, d.chrome),
    text: oneOf(r.text, ["P", "M", "G"] as const, d.text),
    motion: typeof r.motion === "boolean" ? r.motion : d.motion,
    coat: oneOf(r.coat, ["laranja", "cinza", "frajola"] as const, d.coat),
    seen: Array.isArray(r.seen)
      ? r.seen.filter((s): s is string => typeof s === "string").slice(-50)
      : [],
  };
}

export function themeOf(p: Prefs): Theme {
  return THEMES.find((t) => t.id === p.theme) ?? nexora98;
}

export function accentOf(p: Prefs): Accent {
  const t = themeOf(p);
  return t.accents[p.accent] ?? t.accents[0];
}

export function tabbyColors(p: Prefs): TabbyColors {
  const base = themeOf(p).tabby;
  switch (p.coat) {
    case "cinza":
      return { ...base, o: "#9aa3b5", d: "#646b80", w: "#f4f4f6" };
    case "frajola":
      return { ...base, k: "#0d0b0a", o: "#2b2624", d: "#1a1716", w: "#fdfbf7", e: "#f2c14e" };
    default:
      return base;
  }
}

const TEXT_SCALE: Record<TextSize, string> = { P: "0.92", M: "1", G: "1.12" };

/** cssVars is every token of the personalized theme, ready for a style attribute. */
export function cssVars(p: Prefs): Record<string, string> {
  const t = themeOf(p);
  const a = accentOf(p);
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(t.vars)) out[`--${k}`] = v;
  const tabby = tabbyColors(p);
  for (const [k, v] of Object.entries(tabby)) out[`--tabby-${k}`] = v;
  Object.assign(out, {
    "--accent": a.hex,
    "--on-accent": a.on,
    "--accent-dark": a.dark,
    "--icon-accent": t.id === "pastel" ? a.dark : a.hex,
    "--font-chrome": p.chrome === "pixel" ? t.fonts.chrome : t.fonts.body,
    "--font-body": t.fonts.body,
    "--font-mono": t.fonts.mono,
    "--text-scale": TEXT_SCALE[p.text],
  });
  switch (p.wallpaper) {
    case "liso":
      out["--desk-pattern"] = "none";
      break;
    case "grade":
      out["--desk-pattern"] =
        "linear-gradient(var(--grid) 1px, transparent 1px), linear-gradient(90deg, var(--grid) 1px, transparent 1px)";
      out["--desk-pattern-size"] = "16px 16px";
      break;
  }
  return out;
}
