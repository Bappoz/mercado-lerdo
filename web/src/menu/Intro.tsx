import { type ReactNode, useCallback, useEffect, useState } from "react";
import { Pixels, TabbySprite } from "../os/pixel.tsx";
import { CODE, INTRO_LEGEND, LAPTOP, TOWER, YARN } from "./art.ts";

interface Scene {
  place?: string;
  text: string;
  art: ReactNode;
}

const art = (bitmap: typeof YARN, className = "") => (
  <Pixels bitmap={bitmap} legend={INTRO_LEGEND} scale={12} className={`intro-art ${className}`} />
);

// The company's story (docs/product/universe.md §1), told before the first day.
const SCENES: Scene[] = [
  {
    place: "Belo Horizonte",
    text: "A Dona Lurdes faz crochê desde sempre. Tapetes, bolsas, bichinhos. Tudo à mão, com calma.",
    art: art(YARN, "bob"),
  },
  {
    text: "O filho dela, o Álvaro, começou a vender as peças pela internet. E vendia bem.",
    art: art(LAPTOP, "beat"),
  },
  {
    text: "Mas os marketplaces grandes ficavam com quase tudo em taxas. Para quem vende pouco, não sobrava nada.",
    art: (
      <span className="intro-fees">
        <span className="coin c1" />
        <span className="coin c2" />
        <span className="coin c3" />
        {art(TOWER)}
      </span>
    ),
  },
  {
    text: "No colégio, o Álvaro era o Lerdo: metódico demais. Ele pegou o apelido e fez dele uma marca.",
    art: <img className="intro-mark bob" src="/logo-mark.png" alt="" />,
  },
  {
    text: "Nasceu o Mercado Lerdo, o marketplace dos pequenos vendedores: artesãs, brechós, produtores do interior. Sem pressa. Com quem faz.",
    art: <img className="intro-logo" src="/logo.png" alt="Mercado Lerdo" />,
  },
  {
    text: "O site foi feito às pressas pelo Kleber, um freelancer. Funciona. Ninguém sabe muito bem como. Ninguém mediu nada.",
    art: art(CODE, "glitch"),
  },
  {
    place: "Um coworking, hoje",
    text: "A empresa cabe numa mesa, um sofá e uma gata que apareceu num dia de chuva e ficou. A Tabby.",
    art: (
      <span className="intro-rain">
        <TabbySprite pose="sit" scale={10} />
      </span>
    ),
  },
  {
    text: "Uns 130 visitantes por dia, e tudo funciona. Por enquanto. O Mercado Lerdo acabou de contratar a primeira pessoa de engenharia:",
    art: (
      <span className="intro-you">
        <TabbySprite pose="happy" scale={8} />
        <strong className="intro-you-word">você.</strong>
      </span>
    ),
  },
];

const WHOLE = Number.MAX_SAFE_INTEGER;
const TYPE_MS = 28;
const HOLD_MS = 3800;

/**
 * Intro: the cutscene that opens a new game. Click, Enter or the arrow keys
 * advance (the first press finishes the line); Esc skips it all.
 */
export function Intro({
  motion,
  finalLabel,
  onDone,
}: {
  motion: boolean;
  finalLabel: string;
  onDone: () => void;
}) {
  const [pos, setPos] = useState({ i: 0, typed: motion ? 0 : WHOLE });
  const scene = SCENES[pos.i];
  const full = pos.typed >= scene.text.length;
  const last = pos.i === SCENES.length - 1;

  const advance = useCallback(() => {
    setPos((p) => {
      if (p.typed < SCENES[p.i].text.length) return { ...p, typed: WHOLE };
      if (p.i + 1 >= SCENES.length) return p;
      return { i: p.i + 1, typed: motion ? 0 : WHOLE };
    });
  }, [motion]);

  // The typewriter.
  useEffect(() => {
    if (full) return;
    const id = setInterval(() => setPos((p) => ({ ...p, typed: p.typed + 1 })), TYPE_MS);
    return () => clearInterval(id);
  }, [full]);

  // A finished line holds for a moment, then the story moves on by itself.
  useEffect(() => {
    if (!full || last) return;
    const id = setTimeout(advance, HOLD_MS);
    return () => clearTimeout(id);
  }, [full, last, advance]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onDone();
      else if (e.key === "Enter" || e.key === " " || e.key === "ArrowRight") {
        e.preventDefault();
        if (last && full) onDone();
        else advance();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [advance, onDone, last, full]);

  return (
    <div className="intro" role="dialog" aria-label="Abertura: a história do Mercado Lerdo">
      <div className="intro-bar top" aria-hidden="true" />
      <button
        type="button"
        className="intro-stage"
        key={pos.i}
        onClick={last ? undefined : advance}
        aria-label={full ? "Próxima cena" : "Mostrar o texto todo"}
      >
        <span className="intro-picture">{scene.art}</span>
        <span className="intro-caption">
          {scene.place && <span className="intro-place">{scene.place}</span>}
          <span className="intro-text" aria-live="polite">
            {scene.text.slice(0, pos.typed)}
            {!full && <span className="intro-cursor">▌</span>}
          </span>
        </span>
      </button>
      <div className="intro-bar bottom">
        <ol className="intro-dots" aria-label={`Cena ${pos.i + 1} de ${SCENES.length}`}>
          {SCENES.map((s, i) => (
            <li key={s.text} className={i === pos.i ? "on" : i < pos.i ? "seen" : ""} />
          ))}
        </ol>
        {last && full ? (
          <button type="button" className="menu-btn primary" onClick={onDone}>
            {finalLabel}
          </button>
        ) : (
          <span className="intro-hint">clique ou Enter para continuar</span>
        )}
        <button type="button" className="intro-skip" onClick={onDone}>
          Pular (Esc)
        </button>
      </div>
    </div>
  );
}
