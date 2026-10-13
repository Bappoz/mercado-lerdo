import { describe, expect, it } from "vitest";
import type { GameState } from "./api.ts";
import { SLOTTHY_FRAMES } from "./os/pixel.tsx";
import { slotthyMood, slotthyTip } from "./slotthy.ts";
import { placeBubble, placeSlotthy, TOUR_STEPS, walkMs } from "./tour.ts";

const vp = { w: 1440, h: 900 };
const sloth = { w: 96, h: 96 };

describe("Slotthy's frames", () => {
  it("are all 32×32 and use only her palette", () => {
    for (const [pose, rows] of Object.entries(SLOTTHY_FRAMES)) {
      expect(rows, pose).toHaveLength(32);
      for (const row of rows) {
        expect(row, pose).toHaveLength(32);
        expect(row, pose).toMatch(/^[.kodlwep]+$/);
      }
    }
  });
});

describe("the first-day training", () => {
  it("starts and ends with Slotthy center stage, and every stop has something to say", () => {
    expect(TOUR_STEPS[0].target).toBeNull();
    expect(TOUR_STEPS.at(-1)?.target).toBeNull();
    expect(new Set(TOUR_STEPS.map((s) => s.id)).size).toBe(TOUR_STEPS.length);
    for (const s of TOUR_STEPS) {
      expect(s.text.length, s.id).toBeGreaterThan(40);
      expect(s.text.length, s.id).toBeLessThan(320); // fits a bubble without scrolling
    }
  });
});

describe("placeSlotthy", () => {
  it("stands center stage when there is nothing to show", () => {
    const p = placeSlotthy(null, vp, sloth);
    expect(p.x).toBe(vp.w / 2 - sloth.w / 2);
    expect(p.flip).toBe(false);
  });

  it("stands above a dock icon, to its right, pointing left at it", () => {
    const icon = { x: 600, y: 800, w: 60, h: 64 };
    const p = placeSlotthy(icon, vp, sloth);
    expect(p.y + sloth.h).toBeLessThanOrEqual(icon.y);
    expect(p.x).toBeGreaterThan(icon.x);
    expect(p.flip).toBe(false);
  });

  it("switches sides near the right edge and points the other way", () => {
    const icon = { x: 1380, y: 800, w: 60, h: 64 };
    const p = placeSlotthy(icon, vp, sloth);
    expect(p.x + sloth.w).toBeLessThanOrEqual(vp.w);
    expect(p.x).toBeLessThan(icon.x + icon.w / 2);
    expect(p.flip).toBe(true);
  });

  it("stands below something at the very top", () => {
    const bar = { x: 0, y: 0, w: vp.w, h: 40 };
    expect(placeSlotthy(bar, vp, sloth).y).toBeGreaterThanOrEqual(bar.h);
  });
});

describe("placeBubble", () => {
  it("keeps the bubble on the screen next to Slotthy", () => {
    const bubble = { w: 360, h: 200 };
    for (const at of [
      { x: 0, y: 0 },
      { x: vp.w - sloth.w, y: 0 },
      { x: vp.w / 2, y: vp.h - sloth.h },
    ]) {
      const b = placeBubble({ ...at, ...sloth }, bubble, vp);
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

describe("slotthyMood", () => {
  const phase = (p: "calm" | "incident" | "completed") =>
    ({ session: { phase: p } }) as unknown as GameState;
  it("follows the story", () => {
    expect(slotthyMood(null)).toBe("sit");
    expect(slotthyMood(phase("calm"))).toBe("sit");
    expect(slotthyMood(phase("incident"))).toBe("worried");
    expect(slotthyMood(phase("completed"))).toBe("happy");
  });
});

describe("the story's deadlines", () => {
  const state = (kind: string, goal: string) =>
    ({
      session: { phase: "incident" },
      running: true,
      job: null,
      reading: [],
      clock: { paused: false },
      deadline: {
        id: "d",
        kind,
        title: "A loja caiu",
        from: "Álvaro (fundador)",
        goal,
        until: "2026-09-27T18:05:00Z",
      },
    }) as unknown as GameState;
  it("worry Slotthy in an emergency and turn her tip to the request", () => {
    const s = state("emergencia", "store_up");
    expect(slotthyMood(s)).toBe("worried");
    const tip = slotthyTip(s);
    expect(tip.id).toBe("deadline-d");
    expect(tip.text).toContain("Álvaro pediu");
    expect(tip.text).toContain("Deploy");
  });
});
