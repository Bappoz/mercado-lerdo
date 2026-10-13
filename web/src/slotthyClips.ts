// Slotthy's animations: each clip is a list of stills (os/pixel.tsx) and how
// long each one stays. Pure data and one rule for what she does next;
// os/SlotthyActor.tsx plays them.

import type { SlotthyPose } from "./os/pixel.tsx";
import type { Mood } from "./slotthy.ts";

export type Clip = readonly (readonly [pose: SlotthyPose, ms: number])[];

const times = (clip: Clip, n: number): Clip => Array.from({ length: n }, () => clip).flat();

export const CLIPS = {
  // at rest
  idle: [
    ["sit", 1100],
    ["breathe", 1100],
  ],
  blink: [
    ["blinkHalf", 70],
    ["blink", 120],
    ["blinkHalf", 70],
    ["sit", 600],
  ],
  look: [
    ["lookLeft", 800],
    ["sit", 250],
    ["lookRight", 800],
    ["sit", 350],
  ],
  yawn: [
    ["blinkHalf", 220],
    ["yawn1", 220],
    ["yawn2", 950],
    ["yawn1", 200],
    ["blinkHalf", 260],
    ["sit", 400],
  ],
  scratch: [
    ...times(
      [
        ["scratch1", 180],
        ["scratch2", 180],
      ],
      4,
    ),
    ["sit", 400],
  ],
  // falling asleep takes exactly DOZE_MS, then she sleeps until something happens
  doze: [
    ["blinkHalf", 700],
    ["doze", 900],
    ["blinkHalf", 300],
    ["doze", 1100],
  ],
  sleep: [
    ["sleep1", 900],
    ["sleep2", 900],
    ["sleep3", 900],
  ],
  // a new line in her bubble
  talk: [
    ...times(
      [
        ["talk1", 120],
        ["talk2", 150],
        ["talk1", 120],
        ["sit", 110],
      ],
      3,
    ),
  ],
  wave: [
    ...times(
      [
        ["wave1", 220],
        ["wave2", 220],
      ],
      4,
    ),
  ],
  // the moods of the story
  happy: [
    ["happy1", 320],
    ["happy2", 320],
  ],
  happyTalk: [
    ...times(
      [
        ["happyTalk", 150],
        ["happy2", 150],
      ],
      5,
    ),
  ],
  worried: [
    ["worried1", 420],
    ["worried2", 420],
  ],
  worriedTalk: [
    ...times(
      [
        ["worriedTalk", 150],
        ["worried1", 150],
      ],
      5,
    ),
  ],
  // the first-day training
  point: [
    ["point1", 600],
    ["point2", 600],
  ],
  walk: [
    ["walk1", 170],
    ["walk2", 170],
    ["walk3", 170],
    ["walk4", 170],
  ],
} as const satisfies Record<string, Clip>;

export type ClipName = keyof typeof CLIPS;

/** The still of each mood: what she shows with animations off. */
export const REST: Record<Mood, SlotthyPose> = {
  sit: "sit",
  worried: "worried1",
  happy: "happy1",
};

/** A calm Slotthy left alone this long falls asleep: she is a sloth. */
export const SLEEP_AFTER_MS = 60_000;
const DOZE_MS = CLIPS.doze.reduce((sum, [, ms]) => sum + ms, 0);

/** speakClip is how she says a new line; a greeting comes with a wave. */
export function speakClip(mood: Mood, talkKey: string): ClipName {
  if (mood === "happy") return "happyTalk";
  if (mood === "worried") return "worriedTalk";
  return /(welcome|hello)$/.test(talkKey) ? "wave" : "talk";
}

/**
 * nextClip is what she does once a clip ends: quietMs is how long ago she
 * last spoke, r a number in [0, 1) that picks among the fidgets.
 */
export function nextClip(mood: Mood, quietMs: number, r: number): ClipName {
  if (mood === "worried") return "worried";
  if (mood === "happy") return "happy";
  if (quietMs >= SLEEP_AFTER_MS) return quietMs < SLEEP_AFTER_MS + DOZE_MS ? "doze" : "sleep";
  if (r < 0.45) return "idle";
  if (r < 0.75) return "blink";
  if (r < 0.85) return "look";
  if (r < 0.93) return "yawn";
  return "scratch";
}
