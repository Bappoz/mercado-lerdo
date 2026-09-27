// Tabby's lines: scripted from the game state, never from the scenario's
// solution. What the scenario wants to reveal lives in its hint ladder, which
// Tabby hands out only when asked.

import type { GameState } from "./api.ts";

/** Tabby's mood follows the story: worried in an incident, happy once it is solved. */
export type Mood = "sit" | "worried" | "happy";

export function tabbyMood(s: GameState | null): Mood {
  if (s?.session?.phase === "incident") return "worried";
  if (s?.session?.phase === "completed") return "happy";
  return "sit";
}

export interface Tip {
  id: string; // changes when the situation changes, so the bubble reopens
  text: string;
}

export function tabbyTip(s: GameState | null): Tip {
  if (!s) return { id: "loading", text: "Ligando o computador…" };
  const job = s.job;
  if (job?.running) {
    const what = {
      play: "a loja subir",
      check: "o Quick Check",
      test: "o Production Test",
      deploy: "o deploy",
      lab: "o experimento no laboratório",
      advance: "a passagem para o próximo cenário",
    }[job.kind];
    return {
      id: `job-${job.id}`,
      text: `Estou acompanhando ${what}. Pode continuar investigando enquanto isso.`,
    };
  }
  if (!s.session) {
    return {
      id: "welcome",
      text: "O e-mail do RH está na Caixa de entrada: é por ele que o seu primeiro dia começa. Se quiser, o treinamento está em Configurações.",
    };
  }
  if (!s.running) {
    return {
      id: "down",
      text: "A loja está desligada. Ligue de novo na Caixa de entrada: o seu código continua no workspace.",
    };
  }
  const last = job && !job.running ? job : null;
  if (last?.error) {
    return {
      id: `err-${last.id}`,
      text: "Isso não deu certo. A mensagem de erro está no app Testes; ela costuma dizer onde olhar.",
    };
  }
  switch (s.session.phase) {
    case "calm":
      return {
        id: "calm",
        text: "Dia tranquilo. Dê uma volta: o Pulse mostra a produção e o Terminal já tem o psql conectado. Anote o que parecer normal: vai servir de comparação.",
      };
    case "completed":
      return {
        id: "done",
        text: "Você resolveu! O debrief na Caixa de entrada compara a sua solução com outras. Quando quiser, Missões leva você ao próximo cenário.",
      };
  }
  if (last?.kind === "test" && last.test && !last.test.passed) {
    return {
      id: `fail-${last.id}`,
      text: "Ainda não passou. Cada dimensão do resultado diz o que foi medido; comece pela que reprovou.",
    };
  }
  if (last?.kind === "check" && last.check) {
    return {
      id: `check-${last.id}`,
      text: "O Quick Check é só um termômetro. Quando confiar na mudança, rode o Production Test: é ele que vale.",
    };
  }
  return {
    id: "incident",
    text: "Algo mudou. Compare o Pulse de agora com o que você anotou no dia calmo, e leia o Suporte: o que os clientes estão sentindo?",
  };
}
