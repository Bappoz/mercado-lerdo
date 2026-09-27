// The first-day training: Tabby walks the new hire around LerdOS, one stop
// per part of the desktop (docs/product/universe.md). Pure data and geometry;
// os/Tour.tsx animates it.

import type { AppId } from "./theme.ts";

export interface TourStep {
  id: string;
  /** What Tabby stands next to: a dock app, the top bar, or nothing (center stage). */
  target: { app: AppId } | "topbar" | null;
  text: string;
}

export const TOUR_STEPS: TourStep[] = [
  {
    id: "hello",
    target: null,
    text: "Oi! Eu sou a **Tabby**, a gata do escritório. Apareci no coworking num dia de chuva e fiquei. Antes do seu primeiro dia no **Mercado Lerdo**, deixa eu te mostrar o LerdOS: é aqui que você vai trabalhar.",
  },
  {
    id: "topbar",
    target: "topbar",
    text: "Aqui em cima ficam a missão de agora e o estado da loja. Quando alguma coisa quebrar, aparece um aviso de **incidente**. Aí é com você.",
  },
  {
    id: "inbox",
    target: { app: "inbox" },
    text: "A **Caixa de entrada** é por onde a empresa fala com você: o Álvaro, nosso fundador, a Jéssica do suporte, o Otávio do financeiro. Toda missão começa por um e-mail.",
  },
  {
    id: "missions",
    target: { app: "missions" },
    text: "Em **Missões** fica a sua jornada. Cada missão é um problema de verdade do Mercado Lerdo, e o seu código segue de uma para a outra.",
  },
  {
    id: "pulse",
    target: { app: "pulse" },
    text: "O **Pulse** mostra a produção agora: tráfego, erros, lentidão, CPU. Dê uma olhada quando tudo estiver calmo, para saber o que é normal.",
  },
  {
    id: "terminal",
    target: { app: "terminal" },
    text: "O **Terminal** abre um shell dentro da rede da loja, com o `psql` já conectado ao banco. Investigar é aqui.",
  },
  {
    id: "support",
    target: { app: "support" },
    text: "No **Suporte** chegam as reclamações dos clientes. Eles não mentem: se alguém foi cobrado e não recebeu, aparece aqui.",
  },
  {
    id: "tests",
    target: { app: "tests" },
    text: "**Testes** tem dois botões. O Quick Check é um termômetro rápido. O **Production Test** é a prova que vale: carga de verdade, várias vezes, e uma nota por propriedade.",
  },
  {
    id: "infra",
    target: { app: "architecture" },
    text: "**Arquitetura**, **Deploy** e **Docs** cuidam da infraestrutura: o que roda, quanto custa e como colocar no ar. O Otávio libera um orçamento por mês, em dólar. Passou dele, reprova em custo.",
  },
  {
    id: "lab",
    target: { app: "lab" },
    text: "O **Lab** é o seu laboratório de carga: ensaie tráfego, derrube uma instância, veja o que acontece. Sem nota, sem medo.",
  },
  {
    id: "git",
    target: { app: "git" },
    text: "O **Git** mostra o que você mudou e guarda o histórico da sua jornada. Commit é ponto de volta.",
  },
  {
    id: "notes",
    target: { app: "notes" },
    text: "**Notas** é o seu caderno de investigação. E em **Config.** você muda o tema, as cores e até a minha pelagem.",
  },
  {
    id: "me",
    target: null,
    text: "Quando travar, clique em mim: eu dou **uma dica por vez**, sem entregar a resposta. Agora vá à Caixa de entrada: o RH deixou um e-mail para você começar. Bom primeiro dia!",
  },
];

export interface Rect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface Size {
  w: number;
  h: number;
}

const MARGIN = 12;

/**
 * placeTabby is where Tabby stands to present target (null: center stage),
 * and whether her sprite is flipped. She stands beside the target, not on
 * it, and her pointing paw (drawn raised on her left) faces it.
 */
export function placeTabby(
  target: Rect | null,
  vp: Size,
  cat: Size,
): { x: number; y: number; flip: boolean } {
  const clampX = (x: number) => Math.min(Math.max(x, MARGIN), vp.w - cat.w - MARGIN);
  const clampY = (y: number) => Math.min(Math.max(y, MARGIN), vp.h - cat.h - MARGIN);
  if (!target) return { x: clampX(vp.w / 2 - cat.w / 2), y: clampY(vp.h * 0.42), flip: false };
  const above = target.y - cat.h - 4;
  const below = target.y + target.h + 4;
  const y = clampY(above >= MARGIN ? above : below);
  // Stand to the right of the target and point left at it; near the right
  // edge, stand on its left and point right (flipped).
  const right = target.x + target.w * 0.5 + 6;
  if (right + cat.w + MARGIN <= vp.w) return { x: clampX(right), y, flip: false };
  return { x: clampX(target.x + target.w * 0.5 - cat.w - 6), y, flip: true };
}

/** placeBubble keeps the speech bubble next to Tabby and on the screen. */
export function placeBubble(cat: Rect, bubble: Size, vp: Size): { x: number; y: number } {
  const x = Math.min(Math.max(cat.x + cat.w / 2 - bubble.w / 2, MARGIN), vp.w - bubble.w - MARGIN);
  const above = cat.y - bubble.h - 10;
  const y = above >= MARGIN ? above : Math.min(cat.y + cat.h + 10, vp.h - bubble.h - MARGIN);
  return { x, y: Math.max(y, MARGIN) };
}

/** walkMs is how long Tabby takes to walk a distance: a steady pace, never a teleport nor a crawl. */
export function walkMs(dx: number, dy: number): number {
  return Math.round(Math.min(Math.max(Math.hypot(dx, dy) / 0.7, 350), 1600));
}
