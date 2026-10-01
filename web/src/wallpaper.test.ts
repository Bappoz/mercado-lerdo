import { describe, expect, it } from "vitest";
import { cssVars, DEFAULT_PREFS, normalizePrefs, THEMES } from "./theme.ts";
import { houndstooth, tile, WALLPAPERS, wallpaperVars } from "./wallpaper.ts";

const desk = { desk: "#2f6d68", grid: "#3a817a" };

describe("wallpapers", () => {
  it("every wallpaper paints as many sizes, positions and repeats as images", () => {
    for (const w of WALLPAPERS) {
      const v = wallpaperVars(w.id, desk);
      if (w.id === "cena") {
        // The theme's own desk: its color and pattern stay.
        expect(Object.keys(v)).toEqual(["--desk-pattern-position", "--desk-pattern-repeat"]);
        continue;
      }
      // Layers are split by the commas outside any parenthesis (url(...), gradients).
      const layers = (s: string) => {
        let depth = 0;
        let n = 1;
        for (const c of s) {
          if (c === "(") depth++;
          else if (c === ")") depth--;
          else if (c === "," && depth === 0) n++;
        }
        return n;
      };
      const n = layers(v["--desk-pattern"]);
      for (const k of ["size", "position", "repeat"]) {
        expect(layers(v[`--desk-pattern-${k}`]), `${w.id} ${k}`).toBe(n);
      }
    }
  });

  it("the theme's desk stays under a wallpaper that does not bring its own color", () => {
    expect(wallpaperVars("logo", desk)["--desk"]).toBeUndefined();
    expect(wallpaperVars("nuvens", desk)["--desk"]).toBeDefined();
  });

  it("a stored wallpaper survives, and an unknown one falls back to the theme's scene", () => {
    for (const w of WALLPAPERS) expect(normalizePrefs({ wallpaper: w.id }).wallpaper).toBe(w.id);
    expect(normalizePrefs({ wallpaper: "bliss.bmp" }).wallpaper).toBe("cena");
  });

  it("cssVars carries the wallpaper on top of every theme", () => {
    for (const t of THEMES) {
      const v = cssVars({ ...DEFAULT_PREFS, theme: t.id, wallpaper: "ladrilho" });
      expect(v["--desk-pattern"]).toContain("/logo-mark.png");
      expect(v["--desk"]).toBe(t.vars.desk);
    }
  });
});

describe("pixel tiles", () => {
  it("draws one rect per run of pixels, as a data URI", () => {
    const uri = tile(["ab", "aa"], { a: "#111", b: "#222" }, 4);
    expect(uri.startsWith('url("data:image/svg+xml,')).toBe(true);
    const svg = decodeURIComponent(uri.slice('url("data:image/svg+xml,'.length, -2));
    expect(svg).toContain('width="8" height="8"');
    expect(svg.match(/<rect/g)).toHaveLength(3); // a, b, aa
  });

  it("the houndstooth is a 2/2 twill of four dark and four light threads", () => {
    const rows = houndstooth();
    expect(rows).toHaveLength(8);
    expect(rows.every((r) => r.length === 8)).toBe(true);
    // Half the cloth is dark, and the dark block's corner is solid.
    expect(rows.join("").split("#")).toHaveLength(33);
    expect(rows[0].slice(0, 2)).toBe("##");
  });
});
