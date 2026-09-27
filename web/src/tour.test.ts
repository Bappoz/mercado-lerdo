import { describe, expect, it } from "vitest";
import type { GameState } from "./api.ts";
import { TABBY_FRAMES } from "./os/pixel.tsx";
import { tabbyMood } from "./tabby.ts";
import { placeBubble, placeTabby, TOUR_STEPS, walkMs } from "./tour.ts";

const vp = { w: 1440, h: 900 };
const cat = { w: 96, h: 96 };

describe("Tabby's frames", () => {
  it("are all 24×24 and use only her palette", () => {
    for (const [pose, rows] of Object.entries(TABBY_FRAMES)) {
      expect(rows, pose).toHaveLength(24);
      for (const row of rows) {
        expect(row, pose).toHaveLength(24);
        expect(row, pose).toMatch(/^[.kodwep]+$/);
      }
    }
  });
});

describe("the first-day training", () => {
  it("starts and ends with Tabby center stage, and every stop has something to say", () => {
    expect(TOUR_STEPS[0].target).toBeNull();
    expect(TOUR_STEPS.at(-1)?.target).toBeNull();
    expect(new Set(TOUR_STEPS.map((s) => s.id)).size).toBe(TOUR_STEPS.length);
    for (const s of TOUR_STEPS) {
      expect(s.text.length, s.id).toBeGreaterThan(40);
      expect(s.text.length, s.id).toBeLessThan(320); // fits a bubble without scrolling
    }
  });
});

describe("placeTabby", () => {
  it("stands center stage when there is nothing to show", () => {
    const p = placeTabby(null, vp, cat);
    expect(p.x).toBe(vp.w / 2 - cat.w / 2);
    expect(p.flip).toBe(false);
  });

  it("stands above a dock icon, to its right, pointing left at it", () => {
    const icon = { x: 600, y: 800, w: 60, h: 64 };
    const p = placeTabby(icon, vp, cat);
    expect(p.y + cat.h).toBeLessThanOrEqual(icon.y);
    expect(p.x).toBeGreaterThan(icon.x);
    expect(p.flip).toBe(false);
  });

  it("switches sides near the right edge and points the other way", () => {
    const icon = { x: 1380, y: 800, w: 60, h: 64 };
    const p = placeTabby(icon, vp, cat);
    expect(p.x + cat.w).toBeLessThanOrEqual(vp.w);
    expect(p.x).toBeLessThan(icon.x + icon.w / 2);
    expect(p.flip).toBe(true);
  });

  it("stands below something at the very top", () => {
    const bar = { x: 0, y: 0, w: vp.w, h: 40 };
    expect(placeTabby(bar, vp, cat).y).toBeGreaterThanOrEqual(bar.h);
  });
});

describe("placeBubble", () => {
  it("keeps the bubble on the screen next to Tabby", () => {
    const bubble = { w: 360, h: 200 };
    for (const at of [
      { x: 0, y: 0 },
      { x: vp.w - cat.w, y: 0 },
      { x: vp.w / 2, y: vp.h - cat.h },
    ]) {
      const b = placeBubble({ ...at, ...cat }, bubble, vp);
      expect(b.x).toBeGreaterThanOrEqual(0);
      expect(b.y).toBeGreaterThanOrEqual(0);
      expect(b.x + bubble.w).toBeLessThanOrEqual(vp.w);
      expect(b.y + bubble.h).toBeLessThanOrEqual(vp.h);
    }
  });
});

describe("walkMs", () => {
  it("never teleports nor crawls", () => {
    expect(walkMs(0, 1)).toBe(350);
    expect(walkMs(5000, 0)).toBe(1600);
    expect(walkMs(700, 0)).toBe(1000);
  });
});

describe("tabbyMood", () => {
  const phase = (p: "calm" | "incident" | "completed") =>
    ({ session: { phase: p } }) as unknown as GameState;
  it("follows the story", () => {
    expect(tabbyMood(null)).toBe("sit");
    expect(tabbyMood(phase("calm"))).toBe("sit");
    expect(tabbyMood(phase("incident"))).toBe("worried");
    expect(tabbyMood(phase("completed"))).toBe("happy");
  });
});
