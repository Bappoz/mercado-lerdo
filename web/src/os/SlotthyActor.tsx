import { useEffect, useState } from "react";
import type { Mood } from "../slotthy.ts";
import { CLIPS, type Clip, type ClipName, nextClip, REST, speakClip } from "../slotthyClips.ts";
import { walkMs } from "../tour.ts";
import { type SlotthyPose, SlotthySprite } from "./pixel.tsx";

/** Slotthy's size on screen: 32×32 art at scale 3. */
export const SLOTH = 96;

/** useAnimate is false when the player turned animations off or the system asks for less motion. */
export function useAnimate(motion: boolean): boolean {
  const [reduced, setReduced] = useState(
    () => matchMedia("(prefers-reduced-motion: reduce)").matches,
  );
  useEffect(() => {
    const mq = matchMedia("(prefers-reduced-motion: reduce)");
    const on = () => setReduced(mq.matches);
    mq.addEventListener("change", on);
    return () => mq.removeEventListener("change", on);
  }, []);
  return motion && !reduced;
}

/**
 * useIdlePose brings a sitting Slotthy to life: she plays one clip after the
 * other (breathing, blinking, a yawn, a scratch; asleep if left alone) and
 * speaks each time talkKey changes (a new line).
 */
export function useIdlePose(mood: Mood, animate: boolean, talkKey: string): SlotthyPose {
  const [pose, setPose] = useState<SlotthyPose>(REST[mood]);

  useEffect(() => {
    if (!animate) return;
    let timer = 0;
    let quiet = 0; // how long since she last spoke
    const play = (name: ClipName, i: number) => {
      const clip: Clip = CLIPS[name];
      const [still, ms] = clip[i];
      setPose(still);
      timer = window.setTimeout(() => {
        quiet += ms;
        if (i + 1 < clip.length) play(name, i + 1);
        else play(nextClip(mood, quiet, Math.random()), 0);
      }, ms);
    };
    play(talkKey ? speakClip(mood, talkKey) : nextClip(mood, 0, Math.random()), 0);
    return () => clearTimeout(timer);
  }, [mood, animate, talkKey]);

  return animate ? pose : REST[mood];
}

/** useLoop plays a clip over and over while on; off, it is the clip's first still. */
function useLoop(name: ClipName, on: boolean): SlotthyPose {
  const [i, setI] = useState(0);
  const clip: Clip = CLIPS[name];

  useEffect(() => {
    if (!on) return;
    const frames: Clip = CLIPS[name];
    let n = 0;
    let timer = 0;
    const step = () => {
      setI(n);
      timer = window.setTimeout(step, frames[n % frames.length][1]);
      n++;
    };
    step();
    return () => clearTimeout(timer);
  }, [name, on]);

  return clip[on ? i % clip.length : 0][0];
}

/**
 * WalkingSlotthy stands at (x, y) in the viewport. When the position changes
 * she walks there, facing the way she goes, and calls onArrive on arrival.
 * Without animation she is simply there.
 */
export function WalkingSlotthy({
  x,
  y,
  flip,
  pointing,
  mood,
  talkKey,
  animate,
  onArrive,
}: {
  x: number;
  y: number;
  flip: boolean;
  pointing: boolean;
  mood: Mood;
  talkKey: string;
  animate: boolean;
  onArrive: () => void;
}) {
  const [walk, setWalk] = useState({ x, y, ms: 0, left: false, n: 0 });
  const [arrived, setArrived] = useState(0); // the last walk (walk.n) that ended
  if (walk.x !== x || walk.y !== y) {
    setWalk({
      x,
      y,
      ms: animate ? walkMs(x - walk.x, y - walk.y) : 0,
      left: x < walk.x,
      n: walk.n + 1,
    });
  }
  const walking = walk.ms > 0 && arrived !== walk.n;
  const idle = useIdlePose(mood, animate, walking ? "" : talkKey);

  // onArrive is the caller's: a new function on each render must not restart the walk.
  // biome-ignore lint/correctness/useExhaustiveDependencies: the walk restarts only when its target does
  useEffect(() => {
    if (walk.n === 0) return;
    if (walk.ms === 0) {
      onArrive();
      return;
    }
    const done = window.setTimeout(() => {
      setArrived(walk.n);
      onArrive();
    }, walk.ms);
    return () => clearTimeout(done);
  }, [walk]);

  const moving = useLoop(walking ? "walk" : "point", animate && (walking || pointing));
  const pose: SlotthyPose = walking || pointing ? moving : idle;
  // The walk art faces right; the pointing arm is raised on her left.
  const mirrored = walking ? walk.left : pointing ? flip : false;
  return (
    <div
      className="slotthy-walker"
      aria-hidden="true"
      style={{
        left: walk.x,
        top: walk.y,
        transition: walk.ms ? `left ${walk.ms}ms linear, top ${walk.ms}ms linear` : "none",
      }}
    >
      <SlotthySprite pose={pose} flip={mirrored} />
    </div>
  );
}
