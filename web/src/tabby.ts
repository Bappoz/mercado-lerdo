// Tabby's lines: scripted from the game state, never from the scenario's
// solution. What the scenario wants to reveal lives in its hint ladder, which
// Tabby hands out only when asked.

import type { GameState, TutorialView } from "./api.ts";

/** Tabby's mood follows the story: worried in an incident, happy once it is solved. */
export type Mood = "sit" | "worried" | "happy";

export function tabbyMood(s: GameState | null): Mood {
  if (s?.deadline?.kind === "emergencia" || s?.session?.phase === "incident") return "worried";
  if (s?.session?.phase === "completed") return "happy";
  return "sit";
}

// What each kind of request asks, in Tabby's words: where to look, not the answer.
const GOAL_TIP: Record<NonNullable<GameState["deadline"]>["goal"], string> = {
  test_pass: "O que conta é o Production Test passar.",
  store_up: "Primeiro, coloque a loja de volta no ar: o Deploy reinicia o app.",
  deploy: "Um deploy do que você tem já resolve.",
  view_pulse: "É só dar uma olhada no Pulse.",
  view_support: "Dê uma olhada no Suporte.",
};

export interface Tip {
  id: string; // changes when the situation changes, so the bubble reopens
  text: string;
  tutorial?: TutorialView; // the guided mode's step (ADR-0024): the bubble shows it instead of text
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
  const failed: Tip | null = last?.error
    ? {
        id: `err-${last.id}`,
        text: "Isso não deu certo. A mensagem de erro está no app Testes; ela costuma dizer onde olhar.",
      }
    : null;
  // In the guided mode Tabby leads: her step comes before the story's requests.
  const t = s.tutorial;
  if (t?.step) {
    return (
      failed ?? {
        id: `tut-${t.step}${t.wrong ? "-wrong" : ""}${t.help ? "-help" : ""}`,
        text: t.text,
        tutorial: t,
      }
    );
  }
  // With the clock stopped to read (ADR-0026), nothing of the story is urgent.
  if (s.clock.paused) {
    return {
      id: "clock-paused",
      text: "O relógio da história está parado: leia com calma, nenhuma mensagem chega e nenhum prazo corre. Para voltar, é só retomar lá em cima (um deploy, um check ou um teste também retomam).",
    };
  }
  const d = s.deadline;
  if (d) {
    const until = new Date(d.until).toLocaleTimeString("pt-BR", {
      hour: "2-digit",
      minute: "2-digit",
    });
    return {
      id: `deadline-${d.id}`,
      text: `${d.from.split(" ")[0]} pediu: "${d.title}", até ${until}. ${GOAL_TIP[d.goal]}`,
    };
  }
  if (failed) return failed;
  switch (s.session.phase) {
    case "calm": {
      const n = s.reading.length;
      const reading =
        n > 0
          ? ` Separei ${n} ${n === 1 ? "leitura" : "leituras"} para esta missão no Docs; dá para parar o relógio enquanto você lê.`
          : "";
      return {
        id: "calm",
        text: `Dia tranquilo. Dê uma volta: o Pulse mostra a produção, e no Terminal o \`psql\` abre o banco. Anote o que parecer normal: vai servir de comparação.${reading}`,
      };
    }
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
