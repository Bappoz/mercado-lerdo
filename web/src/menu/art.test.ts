import { describe, expect, it } from "vitest";
import { CODE, INTRO_LEGEND, LAPTOP, TOWER, YARN } from "./art.ts";

describe("intro art", () => {
  for (const [name, art] of Object.entries({ YARN, LAPTOP, TOWER, CODE })) {
    it(`${name} is a 16×16 grid in the legend's colors`, () => {
      expect(art).toHaveLength(16);
      for (const row of art) {
        expect(row).toHaveLength(16);
        for (const c of row) expect(c === "." || c in INTRO_LEGEND).toBe(true);
      }
    });
  }
});
