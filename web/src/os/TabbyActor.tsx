import { useEffect, useState } from "react";
import type { Mood } from "../tabby.ts";
import { walkMs } from "../tour.ts";
import { type TabbyPose, TabbySprite } from "./pixel.tsx";

/** Tabby's size on screen: 24×24 art at scale 4. */
export const CAT = 96;

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
 * useIdlePose brings a sitting Tabby to life: she blinks, swishes her tail
 * and moves her mouth for a moment each time talkKey changes (a new line).
 */
export function useIdlePose(mood: Mood, animate: boolean, talkKey: string): TabbyPose {
  const [flash, setFlash] = useState<"blink" | "sitTail" | null>(null);
  const [talk, setTalk] = useState({ key: talkKey, at: 0 });
  const [mouth, setMouth] = useState(false);
  // A new line starts the talking (derived during render, not in an effect).
  if (talk.key !== talkKey) setTalk({ key: talkKey, at: Date.now() });

  useEffect(() => {
    if (!animate) return;
    const timers: number[] = [];
    const every = (ms: number, pose: "blink" | "sitTail", hold: number) =>
      window.setInterval(() => {
        setFlash(pose);
        timers.push(window.setTimeout(() => setFlash(null), hold));
      }, ms);
    const ids = [every(4300, "blink", 150), every(7100, "sitTail", 480)];
    return () => {
      for (const id of ids) clearInterval(id);
      for (const id of timers) clearTimeout(id);
    };
  }, [animate]);

  useEffect(() => {
    if (!animate || !talk.key || talk.at === 0) return;
    const id = window.setInterval(() => setMouth((m) => !m), 170);
    const stop = window.setTimeout(() => {
      clearInterval(id);
      setMouth(false);
    }, 1400);
    return () => {
      clearInterval(id);
      clearTimeout(stop);
    };
  }, [talk, animate]);

  if (mouth) return mood === "happy" ? "happy" : "talk";
  if (flash === "blink") return "blink";
  if (flash === "sitTail" && mood === "sit") return "sitTail";
  return mood;
}

/**
 * WalkingTabby stands at (x, y) in the viewport. When the position changes
 * she walks there, facing the way she goes, and calls onArrive on arrival.
 * Without animation she is simply there.
 */
export function WalkingTabby({
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
  const [step, setStep] = useState(false);
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
    const id = window.setInterval(() => setStep((s) => !s), 150);
    const done = window.setTimeout(() => {
      clearInterval(id);
      setArrived(walk.n);
      onArrive();
    }, walk.ms);
    return () => {
      clearInterval(id);
      clearTimeout(done);
    };
  }, [walk]);

  const pose: TabbyPose = walking ? (step ? "walk2" : "walk1") : pointing ? "point" : idle;
  // The walk art faces right; the pointing paw is raised on her left.
  const mirrored = walking ? walk.left : pointing ? flip : false;
  return (
    <div
      className="tabby-walker"
      aria-hidden="true"
      style={{
        left: walk.x,
        top: walk.y,
        transition: walk.ms ? `left ${walk.ms}ms linear, top ${walk.ms}ms linear` : "none",
      }}
    >
      <TabbySprite pose={pose} flip={mirrored} />
    </div>
  );
}
