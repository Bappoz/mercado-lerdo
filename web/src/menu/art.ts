// The intro's pixel art, in the logo's colors (docs/assets/logo.png).

import type { Bitmap } from "../os/pixel.tsx";

export const INTRO_LEGEND: Record<string, string> = {
  N: "#2f2a6b", // navy, the logo's ink
  Y: "#f5cf2b", // yellow
  R: "#e0607e", // yarn
  r: "#b8405e",
  W: "#fbf7ef",
  G: "#9aa0b4",
  K: "#1d1b2e",
  E: "#e5484d", // a bug
  L: "#46a758",
};

/** Dona Lurdes's yarn, and the needle. */
export const YARN: Bitmap = [
  "................",
  "...........N....",
  "..........N.....",
  ".....RRRRRN.....",
  "...RRrRRRrRR....",
  "..RRrRRRRrRRR...",
  "..RrRRRRrRRRR...",
  ".RrRRRRrRRRRrR..",
  ".RRRRRrRRRRrRR..",
  ".RRRRrRRRRrRRR..",
  "..RRrRRRRrRRRR..",
  "..RRRRRRrRRRR.RR",
  "...RRRRrRRRR..R.",
  ".....RRRRR...R..",
  "............R...",
  "................",
];

/** Selling online: a laptop with a heart. */
export const LAPTOP: Bitmap = [
  "................",
  "..NNNNNNNNNNNN..",
  "..NWWWWWWWWWWN..",
  "..NWWRRWWRRWWN..",
  "..NWRRRRRRRRWN..",
  "..NWRRRRRRRRWN..",
  "..NWWRRRRRRWWN..",
  "..NWWWRRRRWWWN..",
  "..NWWWWRRWWWWN..",
  "..NWWWWWWWWWWN..",
  "..NNNNNNNNNNNN..",
  ".NGGGGGGGGGGGGN.",
  "NGGGGGGGGGGGGGGN",
  "NNNNNNNNNNNNNNNN",
  "................",
  "................",
];

/** MegaShop, the giant marketplace that keeps the fees. */
export const TOWER: Bitmap = [
  "......NNNN......",
  "......NGGN......",
  "....NNNNNNNN....",
  "....NGGGGGGN....",
  "....NGYGGYGN....",
  "....NGGGGGGN....",
  "....NGYGGYGN....",
  "....NGGGGGGN....",
  "....NGYGGYGN....",
  "....NGGGGGGN....",
  "....NGYGGYGN....",
  "....NGGGGGGN....",
  "....NGGNNGGN....",
  "....NGGNNGGN....",
  "..NNNNNNNNNNNN..",
  "................",
];

/** Kleber's code, written in a hurry: bugs included. */
export const CODE: Bitmap = [
  "NNNNNNNNNNNNNNNN",
  "NENYNLNWWWWWWWWN",
  "NNNNNNNNNNNNNNNN",
  "NKKKKKKKKKKKKKKN",
  "NKLLLKYYYYKKKKKN",
  "NKKKYYYYYYYKKKKN",
  "NKKKLLKEEKKKKKKN",
  "NKKYYYYEEYYKKKKN",
  "NKKKKKKKKKKKKKKN",
  "NKLLLLYYYKKKKKKN",
  "NKKKYYYYYYYYKKKN",
  "NKKKKKKKKKKEKEKN",
  "NKLLKYYYKKKKEKKN",
  "NKKKKKKKKKKEKEKN",
  "NNNNNNNNNNNNNNNN",
  "................",
];
