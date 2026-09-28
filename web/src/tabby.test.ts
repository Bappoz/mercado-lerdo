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
