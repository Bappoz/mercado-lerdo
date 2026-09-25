// A stepped line chart in SVG: few points, drawn crisp to match the pixel UI.

import type { Series } from "../api.ts";

export const SERIES_COLORS = ["var(--s1)", "var(--s2)", "var(--s3)", "var(--s4)"];

const W = 560;
const H = 150;
const PAD = { l: 52, r: 8, t: 10, b: 20 };

export function Chart({
  series,
  from,
  to,
  format,
  log,
  floor = 0,
  label,
}: {
  series: Series[];
  from: number;
  to: number;
  format: (v: number) => string;
  log?: boolean; // log scale: latency spans milliseconds to seconds
  floor?: number; // smallest top of the y axis
  label: string;
}) {
  const pts = series.flatMap((s) => s.points ?? []);
  const vmax = Math.max(floor, ...pts.map((p) => p[1]));
  const vmin = log
    ? Math.max(1e-3, Math.min(...pts.map((p) => p[1]).filter((v) => v > 0), vmax))
    : 0;
  const lo = log ? 10 ** Math.floor(Math.log10(vmin)) : 0;
  const hi = log ? 10 ** Math.ceil(Math.log10(Math.max(vmax, lo * 10))) : niceMax(vmax);
  const x = (t: number) => PAD.l + ((t - from) / Math.max(1, to - from)) * (W - PAD.l - PAD.r);
  const y = (v: number) => {
    const f = log
      ? (Math.log10(Math.max(v, lo)) - Math.log10(lo)) / (Math.log10(hi) - Math.log10(lo))
      : v / (hi || 1);
    return H - PAD.b - f * (H - PAD.t - PAD.b);
  };
  const ticks = log
    ? Array.from({ length: Math.round(Math.log10(hi / lo)) + 1 }, (_, i) => lo * 10 ** i)
    : [0, hi / 2, hi];

  return (
    <svg className="chart" viewBox={`0 0 ${W} ${H}`} role="img" aria-label={label}>
      <rect
        x={PAD.l}
        y={PAD.t}
        width={W - PAD.l - PAD.r}
        height={H - PAD.t - PAD.b}
        className="chart-bg"
      />
      {ticks.map((t) => (
        <g key={t}>
          <line x1={PAD.l} x2={W - PAD.r} y1={y(t)} y2={y(t)} className="chart-grid" />
          <text x={PAD.l - 6} y={y(t) + 4} textAnchor="end" className="chart-tick">
            {format(t)}
          </text>
        </g>
      ))}
      <text x={PAD.l} y={H - 4} className="chart-tick">
        {Math.round((to - from) / 60)} min atrás
      </text>
      <text x={W - PAD.r} y={H - 4} textAnchor="end" className="chart-tick">
        agora
      </text>
      {series.map((s, i) => {
        const p = s.points ?? [];
        if (p.length === 0) return null;
        const color = SERIES_COLORS[i % SERIES_COLORS.length];
        if (p.length === 1) {
          // One sample is not a line yet: mark it.
          return (
            <rect
              key={s.name}
              x={x(p[0][0]) - 3}
              y={y(p[0][1]) - 3}
              width={6}
              height={6}
              fill={color}
            />
          );
        }
        // Stepped path: flat between samples, like a pixel plot.
        let d = `M${x(p[0][0]).toFixed(1)},${y(p[0][1]).toFixed(1)}`;
        for (let k = 1; k < p.length; k++) {
          d += `H${x(p[k][0]).toFixed(1)}V${y(p[k][1]).toFixed(1)}`;
        }
        return (
          <path
            key={s.name}
            d={d}
            fill="none"
            stroke={color}
            strokeWidth={2.5}
            shapeRendering="crispEdges"
          />
        );
      })}
    </svg>
  );
}

function niceMax(v: number): number {
  if (v <= 0) return 1;
  const p = 10 ** Math.floor(Math.log10(v));
  for (const m of [1, 2, 2.5, 5, 10]) if (m * p >= v) return m * p;
  return 10 * p;
}

export function Legend({ series, value }: { series: Series[]; value: (s: Series) => string }) {
  return (
    <ul className="legend">
      {series.map((s, i) => (
        <li key={s.name}>
          <span
            className="swatch"
            style={{ background: SERIES_COLORS[i % SERIES_COLORS.length] }}
          />
          {s.name} <strong>{value(s)}</strong>
        </li>
      ))}
    </ul>
  );
}

export const last = (s: Series) => s.points?.at(-1)?.[1];

export function fmtSeconds(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v)) return "—";
  if (v >= 1) return `${v.toFixed(v >= 10 ? 0 : 1)} s`;
  if (v >= 0.001) return `${Math.round(v * 1000)} ms`;
  return `${(v * 1000).toFixed(1)} ms`;
}

export const fmtPct = (v: number | undefined) =>
  v === undefined ? "—" : `${(v * 100).toFixed(v < 0.1 && v > 0 ? 1 : 0)}%`;

export const fmtRate = (v: number | undefined) => (v === undefined ? "—" : `${v.toFixed(1)}/s`);
