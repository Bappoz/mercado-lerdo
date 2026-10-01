import { describe, expect, it } from "vitest";
import type { GameState, TutorialView } from "./api.ts";
import { tabbyTip } from "./tabby.ts";

const step: TutorialView = {
  step: "quem-esta-no-teto",
  index: 6,
  total: 12,
  kind: "pergunta",
  text: "Quem está no teto de CPU?",
  can_help: true,
  choices: [
    { id: "a", text: "o app" },
    { id: "c", text: "o postgres" },
  ],
  continue: false,
  done: false,
};

function state(over: Partial<GameState>): GameState {
  return {
    scenario: { id: "stage1/c1-slow-search", title: "Busca Lenta", version: "0.2.0" },
    session: {
      phase: "incident",
      workspace: "/w",
      started_at: "",
      hints_shown: 0,
      checks: 0,
      tests: 0,
    },
    running: true,
    messages: [],
    hints: [],
    hints_total: 5,
    job: null,
    last_test: null,
    deadline: null,
    career: null,
    tutorial: null,
    reading: [],
    clock: { paused: false },
    ...over,
  };
}

describe("tabbyTip in the guided mode", () => {
  const deadline = {
    id: "live-da-lu",
    kind: "emergencia",
    title: "A Lu vai fazer live",
    from: "Álvaro (fundador)",
    goal: "test_pass" as const,
    until: new Date().toISOString(),
  };

  it("leads before the story's deadline", () => {
    const tip = tabbyTip(state({ tutorial: step, deadline }));
    expect(tip.tutorial?.step).toBe("quem-esta-no-teto");
  });

  it("says a failed job first, then goes back to the step", () => {
    const job = {
      id: 7,
      kind: "check" as const,
      running: false,
      lines: [],
      error: "build",
      started: "",
    };
    expect(tabbyTip(state({ tutorial: step, job })).id).toBe("err-7");
  });

  it("reopens the bubble after a wrong answer", () => {
    const first = tabbyTip(state({ tutorial: step })).id;
    const wrong = tabbyTip(state({ tutorial: { ...step, wrong: "Olhe de novo." } })).id;
    expect(wrong).not.toBe(first);
  });

  it("steps aside once the guided mode is over", () => {
    const over = { ...step, step: "", done: true };
    expect(tabbyTip(state({ tutorial: over, deadline })).id).toBe("deadline-live-da-lu");
  });
});

describe("tabbyTip and the recommended reading (ADR-0026)", () => {
  const reading = [
    {
      id: "explain",
      title: "Como ler um plano",
      source: "Documentação do PostgreSQL",
      url: "https://www.postgresql.org/docs/current/using-explain.html",
      lang: "en" as const,
      minutes: 20,
      why: "Para ler o plano.",
    },
  ];
  const calm = {
    phase: "calm" as const,
    workspace: "/w",
    started_at: "",
    hints_shown: 0,
    checks: 0,
    tests: 0,
  };

  it("points at the reading when a mission starts", () => {
    expect(tabbyTip(state({ session: calm, reading })).text).toContain("1 leitura");
    expect(tabbyTip(state({ session: calm })).text).not.toContain("leitura");
  });

  it("says the clock is stopped instead of counting a deadline", () => {
    const deadline = {
      id: "live-da-lu",
      kind: "emergencia",
      title: "A Lu vai fazer live",
      from: "Álvaro (fundador)",
      goal: "test_pass" as const,
      until: new Date().toISOString(),
    };
    const tip = tabbyTip(
      state({ deadline, clock: { paused: true, since: new Date().toISOString() } }),
    );
    expect(tip.id).toBe("clock-paused");
  });

  it("keeps leading the guided mode while the clock is stopped", () => {
    const tip = tabbyTip(state({ tutorial: step, clock: { paused: true } }));
    expect(tip.tutorial?.step).toBe("quem-esta-no-teto");
  });
});
