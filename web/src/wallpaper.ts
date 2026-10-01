// Wallpapers: what is painted on the desk, under the windows. Like the ones of
// an old Windows (a centered picture, a tile, a two-color pattern), in the
// logo's colors and with its mark. Each one is a stack of CSS backgrounds:
// pictures are sized in % of the desk (so a thumbnail shows the same thing;
// the mark is never stretched past its 256 px), patterns in pixels.

/** Each string is a row; each char a pixel (the same art as os/pixel.tsx). */
type Bitmap = readonly string[];

export const WALLPAPERS = [
  { id: "cena", name: "Cena do tema" },
  { id: "liso", name: "Liso" },
  { id: "grade", name: "Grade de pixels" },
  { id: "logo", name: "Logo ao centro" },
  { id: "ladrilho", name: "Lado a lado" },
  { id: "nuvens", name: "Nuvens" },
  { id: "colina", name: "Colina" },
  { id: "instalacao", name: "Instalação" },
  { id: "croche", name: "Crochê" },
  { id: "xadrez", name: "Pied-de-poule" },
] as const;

export type Wallpaper = (typeof WALLPAPERS)[number]["id"];

export const WALLPAPER_IDS: readonly Wallpaper[] = WALLPAPERS.map((w) => w.id);

// The logo's colors (docs/assets/logo.png), and Dona Lurdes's yarn.
const NAVY = "#2f2a6b";
const NIGHT = "#17143a";
const YARN = "#e0607e";
const MARK = 'url("/logo-mark.png")';

interface Layer {
  image: string;
  size?: string;
  position?: string;
  repeat?: string;
}

/** tile draws a bitmap as an SVG background: one rect per run of equal pixels. */
export function tile(bitmap: Bitmap, legend: Record<string, string>, scale: number): string {
  const w = bitmap[0]?.length ?? 0;
  let rects = "";
  bitmap.forEach((row, y) => {
    for (let x = 0; x < w; ) {
      let end = x + 1;
      while (end < w && row[end] === row[x]) end++;
      const fill = legend[row[x]];
      if (fill) rects += `<rect x="${x}" y="${y}" width="${end - x}" height="1" fill="${fill}"/>`;
      x = end;
    }
  });
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" width="${w * scale}" height="${bitmap.length * scale}" ` +
    `viewBox="0 0 ${w} ${bitmap.length}" shape-rendering="crispEdges">${rects}</svg>`;
  return `url("data:image/svg+xml,${encodeURIComponent(svg)}")`;
}

/** The pied-de-poule: a 2/2 twill woven with four dark and four light threads. */
export function houndstooth(): string[] {
  const dark = (i: number) => i % 8 < 4;
  return Array.from({ length: 8 }, (_, y) =>
    Array.from({ length: 8 }, (_, x) => {
      const warpOnTop = (x + y) % 4 < 2;
      return (warpOnTop ? dark(x) : dark(y)) ? "#" : ".";
    }).join(""),
  );
}

// Knit stitches: a V per stitch, stacked.
const STITCH: Bitmap = ["a......a", "ba....ab", ".ba..ab.", "..baab.."];

// Two clouds on a 64×40 sky, drawn the way the pastel scene's clouds are: lumps on a flat base.
const CLOUDS: Bitmap = [
  "................................................................",
  "................................................................",
  ".........wwww...................................................",
  ".......wwwwwwww.................................................",
  "......wwwwwwwwwwwwww............................................",
  "....wwwwwwwwwwwwwwwwww..........................................",
  "...wwwwwwwwwwwwwwwwwwwww........................................",
  "..wwwwwwwwwwwwwwwwwwwwwwww......................................",
  "..ssssssssssssssssssssssss......................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "...........................................wwww.................",
  ".......................................wwwwwwwwww...............",
  "....................................wwwwwwwwwwwwwww.............",
  "..................................wwwwwwwwwwwwwwwwwww...........",
  ".................................wwwwwwwwwwwwwwwwwwwwww.........",
  ".................................ssssssssssssssssssssss.........",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
  "................................................................",
];

/** bands is a vertical gradient in hard steps, like a 16-color setup screen. */
function bands(from: [number, number, number], to: [number, number, number], n: number): string {
  const stops = Array.from({ length: n }, (_, i) => {
    const c = from.map((f, k) => Math.round(f + ((to[k] - f) * i) / (n - 1)));
    return `rgb(${c.join(" ")}) ${(i * 100) / n}% ${((i + 1) * 100) / n}%`;
  });
  return `linear-gradient(${stops.join(", ")})`;
}

/** A hill: the top of a wide ellipse rising from the bottom of the desk. */
function hill(color: string, x: string, width: string, height: string): string {
  return `radial-gradient(ellipse ${width} ${height} at ${x} 100%, ${color} 99.5%, transparent 100%)`;
}

function layersOf(
  w: Wallpaper,
  t: { desk: string; grid: string },
): { color?: string; layers: Layer[] } {
  switch (w) {
    case "cena":
    case "liso":
      return { layers: [] };
    case "grade":
      return {
        layers: [
          { image: `linear-gradient(${t.grid} 1px, transparent 1px)`, size: "16px 16px" },
          { image: `linear-gradient(90deg, ${t.grid} 1px, transparent 1px)`, size: "16px 16px" },
        ],
      };
    case "logo":
      return {
        layers: [
          { image: MARK, size: "auto min(34%, 256px)", position: "center", repeat: "no-repeat" },
          { image: tile(["g.", ".."], { g: t.grid }, 4) },
        ],
      };
    case "ladrilho":
      return { layers: [{ image: MARK, size: "128px 128px", position: "center" }] };
    case "nuvens":
      return {
        color: "#8ec3ee",
        layers: [
          { image: MARK, size: "auto min(30%, 256px)", position: "center", repeat: "no-repeat" },
          { image: tile(CLOUDS, { w: "#ffffff", s: "#d3e8f8" }, 6) },
          { image: "linear-gradient(#5b9bd9, #b9dcf5)", repeat: "no-repeat" },
        ],
      };
    case "colina":
      return {
        color: "#8ec3ee",
        layers: [
          // The mark is the sun.
          { image: MARK, size: "auto 22%", position: "84% 14%", repeat: "no-repeat" },
          { image: hill("#4f9a3c", "22%", "70%", "46%"), repeat: "no-repeat" },
          { image: hill("#3d7f31", "86%", "62%", "34%"), repeat: "no-repeat" },
          { image: "linear-gradient(#4a8fd6, #bfe0f6 70%)", repeat: "no-repeat" },
        ],
      };
    case "instalacao":
      return {
        color: NIGHT,
        layers: [
          { image: MARK, size: "auto 20%", position: "4% 12%", repeat: "no-repeat" },
          { image: bands([63, 56, 143], [9, 8, 24], 16), repeat: "no-repeat" },
        ],
      };
    case "croche":
      return { color: YARN, layers: [{ image: tile(STITCH, { a: "#f08aa2", b: "#b8405e" }, 4) }] };
    case "xadrez":
      return { color: NAVY, layers: [{ image: tile(houndstooth(), { "#": "#211d52" }, 6) }] };
  }
}

/**
 * wallpaperVars is the desk's tokens for a wallpaper, on top of the theme's
 * (desk and grid are the theme's colors). The theme's scene keeps the theme's pattern.
 */
export function wallpaperVars(
  w: Wallpaper,
  t: { desk: string; grid: string },
): Record<string, string> {
  // Spelled out even for the theme's own desk: a thumbnail must not inherit the desk around it.
  if (w === "cena") return { "--desk-pattern-position": "0 0", "--desk-pattern-repeat": "repeat" };
  const { color, layers } = layersOf(w, t);
  const join = (f: (l: Layer) => string) => layers.map(f).join(", ");
  const out: Record<string, string> = {
    "--desk-pattern": layers.length > 0 ? join((l) => l.image) : "none",
    "--desk-pattern-size": join((l) => l.size ?? "auto") || "auto",
    "--desk-pattern-position": join((l) => l.position ?? "0 0") || "0 0",
    "--desk-pattern-repeat": join((l) => l.repeat ?? "repeat") || "repeat",
  };
  if (color) out["--desk"] = color;
  return out;
}
