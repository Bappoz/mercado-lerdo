import { describe, expect, it } from "vitest";
import { DOCK_SPACE, type Frame, MIN_H, MIN_W, minSize, resizeFrame, TOP_BAR } from "./windows.tsx";

const vp = { w: 1440, h: 900 };
const start: Frame = { open: true, x: 400, y: 200, w: 600, h: 400, z: 1, max: false };
const bottom = vp.h - DOCK_SPACE; // the desktop ends above the dock

describe("resizeFrame", () => {
  it("grows to the right and down from the east and south edges", () => {
    expect(resizeFrame(start, "e", 100, 50, vp)).toMatchObject({ x: 400, w: 700, y: 200, h: 400 });
    expect(resizeFrame(start, "s", 100, 50, vp)).toMatchObject({ x: 400, w: 600, y: 200, h: 450 });
  });

  it("keeps the opposite edge still when pulled from the west or north", () => {
    const w = resizeFrame(start, "w", -100, 0, vp);
    expect(w).toMatchObject({ x: 300, w: 700 });
    expect(w.x + w.w).toBe(start.x + start.w);
    const n = resizeFrame(start, "n", 0, 60, vp);
    expect(n).toMatchObject({ y: 260, h: 340 });
    expect(n.y + n.h).toBe(start.y + start.h);
  });

  it("moves both axes from a corner", () => {
    expect(resizeFrame(start, "nw", -50, -40, vp)).toMatchObject({
      x: 350,
      y: 160,
      w: 650,
      h: 440,
    });
    expect(resizeFrame(start, "se", 30, 20, vp)).toMatchObject({ x: 400, y: 200, w: 630, h: 420 });
  });

  it("stops at the minimum size without sliding the fixed edge", () => {
    const e = resizeFrame(start, "e", -1000, 0, vp);
    expect(e).toMatchObject({ x: 400, w: MIN_W });
    const w = resizeFrame(start, "w", 1000, 0, vp);
    expect(w.w).toBe(MIN_W);
    expect(w.x + w.w).toBe(start.x + start.w);
    const n = resizeFrame(start, "n", 0, 1000, vp);
    expect(n.h).toBe(MIN_H);
    expect(n.y + n.h).toBe(start.y + start.h);
  });

  it("stays on the desktop: not over the top bar, the dock or past the sides", () => {
    expect(resizeFrame(start, "n", 0, -1000, vp).y).toBe(TOP_BAR + 8);
    const s = resizeFrame(start, "s", 0, 1000, vp);
    expect(s.y + s.h).toBe(bottom);
    const e = resizeFrame(start, "e", 5000, 0, vp);
    expect(e.x + e.w).toBe(vp.w - 8);
    expect(resizeFrame(start, "w", -5000, 0, vp).x).toBe(8);
  });

  it("respects an app's own minimum, and a viewport smaller than it", () => {
    const inbox = minSize("inbox");
    expect(inbox.w).toBeGreaterThan(MIN_W);
    expect(resizeFrame(start, "e", -1000, 0, vp, inbox).w).toBe(inbox.w);
    const tiny = { w: 400, h: 500 };
    const f = resizeFrame({ ...start, x: 8, w: 384 }, "e", -1000, 0, tiny, inbox);
    expect(f.w).toBe(tiny.w - 16);
  });
});
