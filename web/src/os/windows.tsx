import { type ReactNode, useCallback, useEffect, useRef, useState } from "react";
import type { AppId } from "../theme.ts";

export interface Frame {
  open: boolean;
  x: number;
  y: number;
  w: number;
  h: number;
  z: number;
  max: boolean;
}

export const TOP_BAR = 40;
export const DOCK_SPACE = 118;
export const MIN_W = 280;
export const MIN_H = 180;

// Designed on a 1440×900 desktop; fitted to the real viewport at open.
const LAYOUT: Record<AppId, { x: number; y: number; w: number; h: number }> = {
  inbox: { x: 32, y: 56, w: 540, h: 460 },
  pulse: { x: 600, y: 56, w: 640, h: 500 },
  terminal: { x: 32, y: 440, w: 720, h: 330 },
  tests: { x: 640, y: 96, w: 620, h: 560 },
  architecture: { x: 120, y: 64, w: 720, h: 560 },
  deploy: { x: 700, y: 120, w: 560, h: 520 },
  support: { x: 1000, y: 64, w: 400, h: 520 },
  lab: { x: 180, y: 90, w: 700, h: 600 },
  missions: { x: 300, y: 70, w: 640, h: 560 },
  git: { x: 140, y: 64, w: 900, h: 600 },
  docs: { x: 200, y: 80, w: 820, h: 600 },
  notes: { x: 1000, y: 420, w: 380, h: 320 },
  profile: { x: 820, y: 64, w: 520, h: 600 },
  settings: { x: 240, y: 72, w: 860, h: 640 },
};

// The smallest size each app stays usable at: two-pane apps keep their list
// column and ~280 px to read (styles.css: .inbox, .docs, .settings, .git).
const MIN: Partial<Record<AppId, { w: number; h: number }>> = {
  inbox: { w: 480, h: 260 },
  docs: { w: 480, h: 260 },
  settings: { w: 470, h: 260 },
  git: { w: 580, h: 320 },
  terminal: { w: 360, h: 200 },
};

/** minSize is how small app's window may be pulled. */
export function minSize(app: AppId): { w: number; h: number } {
  return MIN[app] ?? { w: MIN_W, h: MIN_H };
}

const INITIAL: AppId[] = ["inbox"];

function fitFrame(f: Frame, vw: number, vh: number, min = { w: MIN_W, h: MIN_H }): Frame {
  const areaH = vh - TOP_BAR - DOCK_SPACE;
  const w = Math.max(Math.min(min.w, vw - 16), Math.min(f.w, vw - 16));
  const h = Math.max(Math.min(min.h, areaH - 8), Math.min(f.h, areaH - 8));
  const x = Math.max(8, Math.min(f.x, vw - w - 8));
  const y = Math.max(TOP_BAR + 8, Math.min(f.y, TOP_BAR + areaH - h));
  return { ...f, w, h, x, y };
}

/** Which edge or corner of a window is being pulled. */
export type Edge = "n" | "s" | "e" | "w" | "ne" | "nw" | "se" | "sw";

/**
 * resizeFrame is start pulled by (dx, dy) from edge: the opposite edge stays
 * where it was, the window keeps its minimum size and stays on the desktop
 * (below the top bar, above the dock, inside the sides).
 */
export function resizeFrame(
  start: Frame,
  edge: Edge,
  dx: number,
  dy: number,
  vp: { w: number; h: number },
  min: { w: number; h: number } = { w: MIN_W, h: MIN_H },
): Frame {
  const minX = 8;
  const maxX = vp.w - 8;
  const minY = TOP_BAR + 8;
  const maxY = vp.h - DOCK_SPACE;
  let left = start.x;
  let right = start.x + start.w;
  let top = start.y;
  let bottom = start.y + start.h;
  const clamp = (v: number, lo: number, hi: number) => Math.min(Math.max(v, lo), hi);
  // A viewport smaller than the minimum wins: the window just fills it.
  const minW = Math.min(min.w, maxX - minX);
  const minH = Math.min(min.h, maxY - minY);
  if (edge.includes("w")) left = clamp(left + dx, minX, right - minW);
  if (edge.includes("e")) right = clamp(right + dx, left + minW, maxX);
  if (edge.includes("n")) top = clamp(top + dy, minY, bottom - minH);
  if (edge.includes("s")) bottom = clamp(bottom + dy, top + minH, maxY);
  return { ...start, x: left, y: top, w: right - left, h: bottom - top };
}

const EDGES: Edge[] = ["n", "s", "e", "w", "ne", "nw", "se", "sw"];

export interface WindowManager {
  frames: Record<AppId, Frame>;
  compact: boolean; // small screen: every window is maximized
  open: (app: AppId) => void;
  close: (app: AppId) => void;
  toggle: (app: AppId) => void;
  focus: (app: AppId) => void;
  move: (app: AppId, x: number, y: number) => void;
  resize: (app: AppId, from: Frame, edge: Edge, dx: number, dy: number) => void;
  toggleMax: (app: AppId) => void;
  top: AppId | null;
}

export function useWindows(): WindowManager {
  const [vp, setVp] = useState({ w: window.innerWidth, h: window.innerHeight });
  const zTop = useRef(20);
  // Frames hold where the player wants each window; they are fitted to the
  // viewport when rendered, so shrinking the browser never loses the layout.
  const [wanted, setFrames] = useState<Record<AppId, Frame>>(() => {
    const out = {} as Record<AppId, Frame>;
    for (const [app, l] of Object.entries(LAYOUT) as [AppId, (typeof LAYOUT)[AppId]][]) {
      out[app] = { ...l, open: INITIAL.includes(app), z: 10, max: false };
    }
    return out;
  });

  useEffect(() => {
    const onResize = () => setVp({ w: window.innerWidth, h: window.innerHeight });
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);
  const frames = {} as Record<AppId, Frame>;
  for (const app of Object.keys(wanted) as AppId[])
    frames[app] = fitFrame(wanted[app], vp.w, vp.h, minSize(app));

  const patch = useCallback((app: AppId, p: Partial<Frame>) => {
    setFrames((fs) => ({ ...fs, [app]: { ...fs[app], ...p } }));
  }, []);
  const focus = useCallback((app: AppId) => patch(app, { z: ++zTop.current }), [patch]);
  const open = useCallback((app: AppId) => patch(app, { open: true, z: ++zTop.current }), [patch]);
  const close = useCallback((app: AppId) => patch(app, { open: false }), [patch]);

  const top =
    (Object.entries(frames) as [AppId, Frame][])
      .filter(([, f]) => f.open)
      .sort((a, b) => b[1].z - a[1].z)[0]?.[0] ?? null;

  const toggle = useCallback((app: AppId) => {
    setFrames((fs) => {
      const f = fs[app];
      const isTop = Object.values(fs).every((o) => !o.open || o.z <= f.z);
      // A click on the dock brings a buried window forward; on the top one, it hides it.
      if (f.open && isTop) return { ...fs, [app]: { ...f, open: false } };
      return { ...fs, [app]: { ...f, open: true, z: ++zTop.current } };
    });
  }, []);
  const move = useCallback(
    (app: AppId, x: number, y: number) =>
      setFrames((fs) => ({
        ...fs,
        [app]: fitFrame({ ...fs[app], x, y }, vp.w, vp.h, minSize(app)),
      })),
    [vp],
  );
  const resize = useCallback(
    (app: AppId, from: Frame, edge: Edge, dx: number, dy: number) =>
      setFrames((fs) => ({
        ...fs,
        [app]: { ...fs[app], ...pick(resizeFrame(from, edge, dx, dy, vp, minSize(app))) },
      })),
    [vp],
  );
  const toggleMax = useCallback(
    (app: AppId) => setFrames((fs) => ({ ...fs, [app]: { ...fs[app], max: !fs[app].max } })),
    [],
  );

  return { frames, compact: vp.w < 760, open, close, toggle, focus, move, resize, toggleMax, top };
}

const pick = ({ x, y, w, h }: Frame) => ({ x, y, w, h });

export function Window({
  app,
  title,
  frame,
  compact,
  reading,
  titleBar,
  wm,
  children,
}: {
  app: AppId;
  title: string;
  frame: Frame;
  compact: boolean;
  reading?: boolean; // scales with the text-size preference
  titleBar: { bg: string; ink: string };
  wm: WindowManager;
  children: ReactNode;
}) {
  const drag = useRef<{ dx: number; dy: number } | null>(null);
  const grip = useRef<{ edge: Edge; from: Frame; px: number; py: number } | null>(null);
  if (!frame.open) return null;
  const max = frame.max || compact;
  const style = max
    ? {
        left: 8,
        top: TOP_BAR + 8,
        width: "calc(100vw - 16px)",
        height: `calc(100vh - ${TOP_BAR + DOCK_SPACE + 8}px)`,
        zIndex: frame.z,
      }
    : { left: frame.x, top: frame.y, width: frame.w, height: frame.h, zIndex: frame.z };

  return (
    <section
      className="win"
      data-app={app}
      aria-label={title}
      style={style}
      onPointerDownCapture={() => wm.focus(app)}
    >
      {/* biome-ignore lint/a11y/noStaticElementInteractions: dragging is a pointer-only convenience; every window action also has a button */}
      <header
        className="win-bar"
        style={{ background: titleBar.bg, color: titleBar.ink }}
        onPointerDown={(e) => {
          if (max || (e.target as HTMLElement).closest("button")) return;
          drag.current = { dx: e.clientX - frame.x, dy: e.clientY - frame.y };
          e.currentTarget.setPointerCapture(e.pointerId);
        }}
        onPointerMove={(e) => {
          if (drag.current) wm.move(app, e.clientX - drag.current.dx, e.clientY - drag.current.dy);
        }}
        onPointerUp={() => {
          drag.current = null;
        }}
        onDoubleClick={() => !compact && wm.toggleMax(app)}
      >
        <h2 className="win-title">{title}</h2>
        <div className="win-actions">
          {!compact && (
            <button
              type="button"
              className="win-btn"
              aria-label={max ? `Restaurar ${title}` : `Maximizar ${title}`}
              onClick={() => wm.toggleMax(app)}
            >
              {max ? "▭" : "□"}
            </button>
          )}
          <button
            type="button"
            className="win-btn"
            aria-label={`Fechar ${title}`}
            onClick={() => wm.close(app)}
          >
            x
          </button>
        </div>
      </header>
      <div className={reading ? "win-body reading" : "win-body"}>{children}</div>
      {!max &&
        EDGES.map((edge) => (
          <div
            key={edge}
            className={`win-grip win-grip-${edge}`}
            aria-hidden="true"
            onPointerDown={(e) => {
              grip.current = { edge, from: frame, px: e.clientX, py: e.clientY };
              e.currentTarget.setPointerCapture(e.pointerId);
              e.preventDefault(); // no text selection while pulling
            }}
            onPointerMove={(e) => {
              const g = grip.current;
              if (g) wm.resize(app, g.from, g.edge, e.clientX - g.px, e.clientY - g.py);
            }}
            onPointerUp={() => {
              grip.current = null;
            }}
            onPointerCancel={() => {
              grip.current = null;
            }}
          />
        ))}
    </section>
  );
}
