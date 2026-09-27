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
  missions: [
    ".#........",
    ".#aaaaaa..",
    ".#aaaaaaa.",
    ".#aaaaaa..",
    ".#........",
    ".#........",
    ".#........",
    "####......",
  ],
  git: [
    ".a......a.",
    ".#......#.",
    ".#.....#..",
    ".#...##...",
    ".#.##.....",
    ".##.......",
    ".#........",
    ".a........",
  ],
  lab: [
    "..........",
    "........#.",
    "......#.#.",
    "....#.#.#.",
    "..#.#.#.#.",
    "..a.a.a.a.",
    "##########",
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
  profile: [
    "...####...",
    "..#aaaa#..",
    "..#aaaa#..",
    "...####...",
    ".########.",
    "#aaaaaaaa#",
    "#aaaaaaaa#",
    "##########",
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

// Tabby, 24×24: a striped (tabby) cat. Poses for the moods and the walk; the
// walk and the pointing paw face one way and are flipped for the other.
export type TabbyPose =
  | "sit"
  | "sitTail"
  | "blink"
  | "talk"
  | "happy"
  | "worried"
  | "point"
  | "walk1"
  | "walk2";

export const TABBY_FRAMES: Record<TabbyPose, Bitmap> = {
  sit: [
    ".....kk.........kk......",
    "....kook..kkk..kook.....",
    "....koookkoookkoook.....",
    "....kpppododdodpppk.....",
    "...koppoododdodoppok....",
    "...koooooooddooooook....",
    "...koooooooooooooook....",
    "..koooooooooooooooook...",
    "..kooooweooooooweoook...",
    "..koddoeeooooooeeoddk...",
    "...koooeewwppwoeeook....",
    "...kddpowwkwwkwoopdd....",
    "....koooowwkkwooook.kk..",
    ".....koooowwwooook.kook.",
    "......kkoooooookk..kook.",
    "......kooowwwoook..koook",
    ".....kooowwwwwoook.koook",
    "....kodoowwwwwooodkkodok",
    "....koooowwwwwooookooook",
    "....kodoowwwwwooodkoook.",
    "....koooowwwwwoooooodok.",
    ".....koowwowowwooooook..",
    "......kwwwwkkwwwookkk...",
    ".......kkwooowkkkk......",
  ],
  sitTail: [
    ".....kk.........kk......",
    "....kook..kkk..kook.....",
    "....koookkoookkoook.....",
    "....kpppododdodpppk.....",
    "...koppoododdodoppok....",
    "...koooooooddooooook....",
    "...koooooooooooooook....",
    "..koooooooooooooooook...",
    "..kooooweooooooweoook...",
    "..koddoeeooooooeeoddk...",
    "...koooeewwppwoeeook....",
    "...kddpowwkwwkwoopdd....",
    "....koooowwkkwooook.....",
    ".....koooowwwooook....k.",
    "......kkoooooookk....kok",
    "......kooowwwoook...kooo",
    ".....kooowwwwwoook..kooo",
    "....kodoowwwwwooodk.kodo",
    "....koooowwwwwooook.kooo",
    "....kodoowwwwwooodkkoook",
    "....koooowwwwwooooooodok",
    ".....koowwowowwoooooook.",
    "......kwwwwkkwwwoookkk..",
    ".......kkwooowkkkkk.....",
  ],
  blink: [
    ".....kk.........kk......",
    "....kook..kkk..kook.....",
    "....koookkoookkoook.....",
    "....kpppododdodpppk.....",
    "...koppoododdodoppok....",
    "...koooooooddooooook....",
    "...koooooooooooooook....",
    "..koooooooooooooooook...",
    "..koooooooooooooooook...",
    "..koddkookooookookddk...",
    "...koookkwwppwokkook....",
    "...kddpowwkwwkwoopdd....",
    "....koooowwkkwooook.kk..",
    ".....koooowwwooook.kook.",
    "......kkoooooookk..kook.",
    "......kooowwwoook..koook",
    ".....kooowwwwwoook.koook",
    "....kodoowwwwwooodkkodok",
    "....koooowwwwwooookooook",
    "....kodoowwwwwooodkoook.",
    "....koooowwwwwoooooodok.",
    ".....koowwowowwooooook..",
    "......kwwwwkkwwwookkk...",
    ".......kkwooowkkkk......",
  ],
  talk: [
    ".....kk.........kk......",
    "....kook..kkk..kook.....",
    "....koookkoookkoook.....",
    "....kpppododdodpppk.....",
    "...koppoododdodoppok....",
    "...koooooooddooooook....",
    "...koooooooooooooook....",
    "..koooooooooooooooook...",
    "..kooooweooooooweoook...",
    "..koddoeeooooooeeoddk...",
    "...koooeewwppwoeeook....",
    "...kddpowwwkkwwoopdd....",
    "....koooowkppkooook.kk..",
    ".....koooowkkooook.kook.",
    "......kkoooooookk..kook.",
    "......kooowwwoook..koook",
    ".....kooowwwwwoook.koook",
    "....kodoowwwwwooodkkodok",
    "....koooowwwwwooookooook",
    "....kodoowwwwwooodkoook.",
    "....koooowwwwwoooooodok.",
    ".....koowwowowwooooook..",
    "......kwwwwkkwwwookkk...",
    ".......kkwooowkkkk......",
  ],
  happy: [
    ".....kk.........kk......",
    "....kook..kkk..kook.....",
    "....koookkoookkoook.....",
    "....kpppododdodpppk.....",
    "...koppoododdodoppok....",
    "...koooooooddooooook....",
    "...koooooooooooooook....",
    "..koooooooooooooooook...",
    "..koooooooooooooooook...",
    "..koddokkooooookkoddk...",
    "...kookookwppwkookok....",
    "...kddpowwwkkwwoopdd....",
    "....koooowkppkooook.kk..",
    ".....koooowkkooook.kook.",
    "......kkoooooookk..kook.",
    "......kooowwwoook..koook",
    ".....kooowwwwwoook.koook",
    "....kodoowwwwwooodkkodok",
    "....koooowwwwwooookooook",
    "....kodoowwwwwooodkoook.",
    "....koooowwwwwoooooodok.",
    ".....koowwowowwooooook..",
    "......kwwwwkkwwwookkk...",
    ".......kkwooowkkkk......",
  ],
  worried: [
    ".....kk.........kk......",
    "....kook..kkk..kook.....",
    "....koookkoookkoook.....",
    "....kpppododdodpppk.....",
    "...koppoododdodoppokk...",
    "...kooookooddookoookwk..",
    "...kookkooooooookkokwwk.",
    "..kooooooooooooooookwwk.",
    "..kooooweooooooweoookk..",
    "..koddeeeeooooeeeeddk...",
    "...koooeewwppwoeeook....",
    "...kddpowwwkkwwoopdd....",
    "....koooowkwwkooook.kk..",
    ".....koooowwwooook.kook.",
    "......kkoooooookk..kook.",
    "......kooowwwoook..koook",
    ".....kooowwwwwoook.koook",
    "....kodoowwwwwooodkkodok",
    "....koooowwwwwooookooook",
    "....kodoowwwwwooodkoook.",
    "....koooowwwwwoooooodok.",
    ".....koowwowowwooooook..",
    "......kwwwwkkwwwookkk...",
    ".......kkwooowkkkk......",
  ],
  point: [
    ".....kk.........kk......",
    "....kook..kkk..kook.....",
    "....koookkoookkoook.....",
    "....kpppododdodpppk.....",
    "...koppoododdodoppok....",
    "...koooooooddooooook....",
    "...koooooooooooooook....",
    "..koooooooooooooooook...",
    "..kooooweooooooweoook...",
    ".kkoddoeeooooooeeoddk...",
    "kwwkoooeewwppwoeeook....",
    "kwwoddpowwwkkwwoopdd....",
    "koooooooowkppkooook.kk..",
    ".koooooooowkkooook.kook.",
    "..kooookoooooookk..kook.",
    "...koooooowwwoook..koook",
    "....koooowwwwwoook.koook",
    "....kodoowwwwwooodkkodok",
    "....koooowwwwwooookooook",
    "....kodoowwwwwooodkoook.",
    "....koooowwwwwoooooodok.",
    ".....koowwowowwooooook..",
    "......kwwwwkkwwwookkk...",
    ".......kkwooowkkkk......",
  ],
  walk1: [
    "...............k.....k..",
    "..............kok...kok.",
    "..............kookkkook.",
    ".............koopoooook.",
    "...kk........kopododook.",
    "..kook.......kooooodoook",
    ".koook......kooooooooooo",
    "koodk.......kooooooweooo",
    "koook.......kooooooeewoo",
    "koook.......koooooowwwpo",
    "koodk...kkkkkkooooowwkwk",
    "koook.kkdoodoodooooowwk.",
    ".koookoodoodoodoooookk..",
    ".kooooooooooooooookk....",
    "..kooooooooooooooook....",
    "...koooooooooooooook....",
    "...koooooooooooooook....",
    "....koooooooooooook.....",
    ".....kooooooooookook....",
    ".....kookookkkookook....",
    "....koookookkookkook....",
    "....kwwk.kk..kk.kwwk....",
    ".....kk..........kk.....",
    "........................",
  ],
  walk2: [
    "...............k.....k..",
    "..............kok...kok.",
    "..............kookkkook.",
    ".............koopoooook.",
    "...kk........kopododook.",
    "..kook.......kooooodoook",
    ".koook......kooooooooooo",
    "koodk.......kooooooweooo",
    "koook.......kooooooeewoo",
    "koook.......koooooowwwpo",
    "koodk...kkkkkkooooowwkwk",
    "koook.kkdoodoodooooowwk.",
    ".koookoodoodoodoooookk..",
    ".kooooooooooooooookk....",
    "..kooooooooooooooook....",
    "...koooooooooooooook....",
    "...koooooooooooooook....",
    "....koooooooooooook.....",
    ".....kooooooooooook.....",
    ".....kooookkkkooook.....",
    "......koook..kooook.....",
    ".......kwwk...kwwk......",
    "........kk.....kk.......",
    "........................",
  ],
};

const TABBY_LEGEND = {
  k: "var(--tabby-k)",
  o: "var(--tabby-o)",
  d: "var(--tabby-d)",
  w: "var(--tabby-w)",
  e: "var(--tabby-e)",
  p: "var(--tabby-p)",
};

export function TabbySprite({
  pose = "sit",
  flip,
  scale = 4,
}: {
  pose?: TabbyPose;
  flip?: boolean;
  scale?: number;
}) {
  return (
    <Pixels
      bitmap={TABBY_FRAMES[pose]}
      legend={TABBY_LEGEND}
      scale={scale}
      className={flip ? "tabby-art flip" : "tabby-art"}
    />
  );
}
