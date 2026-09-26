// Pixel art drawn as SVG rects: crisp at any zoom, colored by theme tokens.

import type { AppId } from "../theme.ts";

/** Each string is a row; each char a pixel. A legend maps chars to fills. */
export type Bitmap = readonly string[];

export function Pixels({
  bitmap,
  legend,
  scale,
  className,
}: {
  bitmap: Bitmap;
  legend: Record<string, string>;
  scale: number;
  className?: string;
}) {
  const w = bitmap[0]?.length ?? 0;
  const h = bitmap.length;
  const rects = [];
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const fill = legend[bitmap[y][x]];
      if (fill) rects.push(<rect key={`${x}-${y}`} x={x} y={y} width={1} height={1} fill={fill} />);
    }
  }
  return (
    <svg
      className={className}
      width={w * scale}
      height={h * scale}
      viewBox={`0 0 ${w} ${h}`}
      shapeRendering="crispEdges"
      aria-hidden="true"
    >
      {rects}
    </svg>
  );
}

const ICON_LEGEND = { "#": "var(--icon-ink)", a: "var(--icon-accent)" };

export const ICONS: Record<AppId, Bitmap> = {
  inbox: [
    "..........",
    "##########",
    "#a......a#",
    "#.a....a.#",
    "#..a..a..#",
    "#...aa...#",
    "#........#",
    "##########",
  ],
  pulse: [
    "#.........",
    "#.....#...",
    "#....#.#..",
    "#.#.#...#.",
    "#..#.....#",
    "#.........",
    "#.........",
    "##########",
  ],
  terminal: [
    "##########",
    "#aaaaaaaa#",
    "#........#",
    "#.#......#",
    "#..#.....#",
    "#.#..###.#",
    "#........#",
    "##########",
  ],
  tests: [
    "##########",
    "#........#",
    "#......a.#",
    "#.....a..#",
    "#.a..a...#",
    "#..aa....#",
    "#........#",
    "##########",
  ],
  architecture: [
    "...##.....",
    "...##.....",
    "....#.....",
    ".aa.#.aa..",
    ".aa###aa..",
    "....#.....",
    "...###....",
    "...###....",
  ],
  deploy: [
    "....##....",
    "...#aa#...",
    "..#aaaa#..",
    "....##....",
    "....##....",
    "....##....",
    "..######..",
    "..........",
  ],
  support: [
    ".########.",
    "#........#",
    "#.aa..aa.#",
    "#........#",
    "#..####..#",
    ".########.",
    "...##.....",
    "...#......",
  ],
  docs: [
    ".########.",
    ".#aaaaaa#.",
    ".#......#.",
    ".#aaaaa.#.",
    ".#......#.",
    ".#aaaa..#.",
    ".########.",
    "..........",
  ],
  notes: [
    ".#.#.#.#..",
    ".########.",
    ".#......#.",
    ".#.aaaa.#.",
    ".#......#.",
    ".#.aaa..#.",
    ".########.",
    "..........",
  ],
  settings: [
    "...#..#...",
    "..######..",
    ".##.aa.##.",
    "..#a..a#..",
    "..#a..a#..",
    ".##.aa.##.",
    "..######..",
    "...#..#...",
  ],
};

export function AppIcon({ app, scale = 4 }: { app: AppId; scale?: number }) {
  return <Pixels bitmap={ICONS[app]} legend={ICON_LEGEND} scale={scale} />;
}

// Tabby, 16×16. Placeholder sprite until the mascot gets its own art pass.
const TABBY: Bitmap = [
  "..kk........kk..",
  ".kpk........kpk.",
  ".kook......kook.",
  ".koookkkkkkoook.",
  "kooddoooooddoook",
  "kooooooooooooook",
  "kooeeooooooeeook",
  "kooeeooooooeeook",
  "koooowwppwwooook",
  "kodoowkwwkwoodok",
  ".koooowwwwoooook",
  "..kkoooooooookk.",
  ".koodoooooodook.",
  "koooodowwodooook",
  "kooooowwwwoooook",
  ".kkkkkkkkkkkkkk.",
];

// Blink frame: the eyes close to a line.
const TABBY_BLINK: Bitmap = TABBY.map((row, y) =>
  y === 6 ? row.replaceAll("e", "o") : y === 7 ? row.replaceAll("e", "k") : row,
);

const TABBY_LEGEND = {
  k: "var(--tabby-k)",
  o: "var(--tabby-o)",
  d: "var(--tabby-d)",
  w: "var(--tabby-w)",
  e: "var(--tabby-e)",
  p: "var(--tabby-p)",
};

export function TabbySprite({ blink, scale = 5 }: { blink?: boolean; scale?: number }) {
  return <Pixels bitmap={blink ? TABBY_BLINK : TABBY} legend={TABBY_LEGEND} scale={scale} />;
}
