import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Markdown } from "../markdown.tsx";
import { placeBubble, placeTabby, type Rect, TOUR_STEPS, type TourStep } from "../tour.ts";
import { CAT, useAnimate, WalkingTabby } from "./TabbyActor.tsx";

const viewport = () => ({ w: window.innerWidth, h: window.innerHeight });

/** Where Tabby rests between trainings: the corner she sits in (styles.css .tabby). */
export const corner = (vp: { w: number; h: number }) => ({
  x: vp.w - 20 - CAT - 8,
  y: vp.h - 120 - CAT - 8,
  flip: false,
});

function targetRect(step: TourStep): Rect | null {
  const t = step.target;
  if (t === null) return null;
  const el = document.querySelector(t === "topbar" ? ".topbar" : `.dock-item[data-app="${t.app}"]`);
  if (!el) return null;
  const r = el.getBoundingClientRect();
  return { x: r.x, y: r.y, w: r.width, h: r.height };
}

/**
 * Tour is the first-day training: Tabby walks from her corner to each part of
 * LerdOS, points at it and explains it, then walks back. The rest of the
 * desktop is dimmed and does not take clicks until the tour ends or is skipped.
 */
export function Tour({ motion, onDone }: { motion: boolean; onDone: () => void }) {
  const animate = useAnimate(motion);
  const [vp, setVp] = useState(viewport);
  const [i, setI] = useState(0);
  const [entered, setEntered] = useState(false); // she starts in her corner and walks in
  const [leaving, setLeaving] = useState(false);
  const [reached, setReached] = useState<{ x: number; y: number } | null>(null); // where she last arrived
  const [rect, setRect] = useState<Rect | null>(null);
  const [bubbleSize, setBubbleSize] = useState({ w: 340, h: 180 });
  const bubble = useRef<HTMLDivElement>(null);
  const next = useRef<HTMLButtonElement>(null);
  const step = TOUR_STEPS[i];
  const last = i === TOUR_STEPS.length - 1;

  // After the first paint she walks in from the corner. A timer, not
  // requestAnimationFrame: that one never fires while the tab is hidden.
  useEffect(() => {
    const id = window.setTimeout(() => setEntered(true), animate ? 60 : 0);
    return () => clearTimeout(id);
  }, [animate]);
  useEffect(() => {
    const on = () => setVp(viewport());
    window.addEventListener("resize", on);
    return () => window.removeEventListener("resize", on);
  }, []);
  // Measuring the DOM is what layout effects are for.
  // biome-ignore lint/correctness/useExhaustiveDependencies: a resize (vp) moves the dock, so the target is measured again
  useLayoutEffect(() => {
    setRect(targetRect(step));
  }, [step, vp]);
  useLayoutEffect(() => {
    const b = bubble.current?.getBoundingClientRect();
    if (b && (Math.abs(b.width - bubbleSize.w) > 1 || Math.abs(b.height - bubbleSize.h) > 1)) {
      setBubbleSize({ w: b.width, h: b.height });
    }
  });

  const stop = !entered || leaving ? corner(vp) : placeTabby(rect, vp, { w: CAT, h: CAT });
  // Arrival is about the place, not the step: two steps can share a stop.
  const there = reached !== null && reached.x === stop.x && reached.y === stop.y;
  const arrived = entered && !leaving && there;
  useEffect(() => {
    if (leaving && there) onDone();
  }, [leaving, there, onDone]);
  useEffect(() => {
    if (arrived) next.current?.focus();
  }, [arrived]);

  const finish = useCallback(() => setLeaving(true), []);
  const go = useCallback(
    (d: number) => {
      if (d > 0 && last) return finish();
      setI((n) => Math.min(Math.max(n + d, 0), TOUR_STEPS.length - 1));
    },
    [last, finish],
  );
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") finish();
      else if (e.key === "ArrowRight") go(1);
      else if (e.key === "ArrowLeft") go(-1);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [go, finish]);

  const spot =
    rect && !leaving ? { x: rect.x - 6, y: rect.y - 6, w: rect.w + 12, h: rect.h + 12 } : null;
  const at = placeBubble({ x: stop.x, y: stop.y, w: CAT, h: CAT }, bubbleSize, vp);

  return (
    <div className="tour" role="dialog" aria-modal="true" aria-labelledby="tour-title">
      <div className="tour-veil" />
      <div
        className={spot ? "tour-spot" : "tour-spot none"}
        style={spot ? { left: spot.x, top: spot.y, width: spot.w, height: spot.h } : undefined}
      />
      <WalkingTabby
        x={stop.x}
        y={stop.y}
        flip={stop.flip}
        pointing={arrived && step.target !== null}
        mood={last ? "happy" : "sit"}
        talkKey={arrived ? step.id : ""}
        animate={animate}
        onArrive={() => setReached({ x: stop.x, y: stop.y })}
      />
      <div
        ref={bubble}
        className="bubble tour-bubble"
        style={{ left: at.x, top: at.y, visibility: arrived ? "visible" : "hidden" }}
      >
        <p className="bubble-kind" id="tour-title">
          Treinamento do primeiro dia · {i + 1} de {TOUR_STEPS.length}
        </p>
        <Markdown source={step.text} />
        <div className="bubble-actions">
          {i > 0 && (
            <button type="button" className="btn small ghost" onClick={() => go(-1)}>
              Voltar
            </button>
          )}
          <button type="button" className="btn small" ref={next} onClick={() => go(1)}>
            {last ? "Começar o primeiro dia" : "Próximo"}
          </button>
          {!last && (
            <button type="button" className="btn small ghost" onClick={finish}>
              Pular treinamento
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
