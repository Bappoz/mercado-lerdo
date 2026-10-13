import { describe, expect, it } from "vitest";
import type { GameState, TutorialView } from "./api.ts";
import { SLOTTHY_FRAMES } from "./os/pixel.tsx";
import { slotthyTip } from "./slotthy.ts";
import { CLIPS, nextClip, REST, SLEEP_AFTER_MS, speakClip } from "./slotthyClips.ts";

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

describe("slotthyTip in the guided mode", () => {
  const deadline = {
    id: "live-da-lu",
    kind: "emergencia",
    title: "A Lu vai fazer live",
    from: "Álvaro (fundador)",
    goal: "test_pass" as const,
    until: new Date().toISOString(),
  };

  it("leads before the story's deadline", () => {
    const tip = slotthyTip(state({ tutorial: step, deadline }));
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
    expect(slotthyTip(state({ tutorial: step, job })).id).toBe("err-7");
  });

  it("reopens the bubble after a wrong answer", () => {
    const first = slotthyTip(state({ tutorial: step })).id;
    const wrong = slotthyTip(state({ tutorial: { ...step, wrong: "Olhe de novo." } })).id;
    expect(wrong).not.toBe(first);
  });

  it("steps aside once the guided mode is over", () => {
    const over = { ...step, step: "", done: true };
    expect(slotthyTip(state({ tutorial: over, deadline })).id).toBe("deadline-live-da-lu");
  });
});

describe("slotthyTip and the recommended reading (ADR-0026)", () => {
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
    expect(slotthyTip(state({ session: calm, reading })).text).toContain("1 leitura");
    expect(slotthyTip(state({ session: calm })).text).not.toContain("leitura");
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
    const tip = slotthyTip(
      state({ deadline, clock: { paused: true, since: new Date().toISOString() } }),
    );
    expect(tip.id).toBe("clock-paused");
  });

  it("keeps leading the guided mode while the clock is stopped", () => {
    const tip = slotthyTip(state({ tutorial: step, clock: { paused: true } }));
    expect(tip.tutorial?.step).toBe("quem-esta-no-teto");
  });
});

describe("Slotthy's clips", () => {
  it("use only frames that exist, and every frame is in some clip", () => {
    const used = new Set<string>();
    for (const [name, clip] of Object.entries(CLIPS)) {
      expect(clip.length, name).toBeGreaterThan(0);
      for (const [pose, ms] of clip) {
        expect(SLOTTHY_FRAMES[pose], `${name}: ${pose}`).toBeDefined();
        expect(ms, name).toBeGreaterThan(0);
        used.add(pose);
      }
    }
    expect([...used].sort()).toEqual(Object.keys(SLOTTHY_FRAMES).sort());
  });

  it("rest on a frame of each mood", () => {
    for (const pose of Object.values(REST)) expect(SLOTTHY_FRAMES[pose]).toBeDefined();
  });

  it("follow the mood of the story", () => {
    expect(nextClip("worried", 0, 0)).toBe("worried");
    expect(nextClip("worried", SLEEP_AFTER_MS * 2, 0.99)).toBe("worried");
    expect(nextClip("happy", SLEEP_AFTER_MS * 2, 0.5)).toBe("happy");
    expect(speakClip("worried", "tip-incident")).toBe("worriedTalk");
    expect(speakClip("happy", "tip-done")).toBe("happyTalk");
  });

  it("fidget while calm, mostly breathing and blinking", () => {
    expect(nextClip("sit", 0, 0)).toBe("idle");
    expect(nextClip("sit", 0, 0.5)).toBe("blink");
    expect(nextClip("sit", 0, 0.8)).toBe("look");
    expect(nextClip("sit", 0, 0.9)).toBe("yawn");
    expect(nextClip("sit", 0, 0.99)).toBe("scratch");
  });

  it("doze off once, then sleep, when left alone on a calm day", () => {
    const doze = CLIPS.doze.reduce((sum, [, ms]) => sum + ms, 0);
    expect(nextClip("sit", SLEEP_AFTER_MS - 1, 0)).toBe("idle");
    expect(nextClip("sit", SLEEP_AFTER_MS, 0)).toBe("doze");
    expect(nextClip("sit", SLEEP_AFTER_MS + doze, 0)).toBe("sleep");
  });

  it("wave when she greets, talk otherwise", () => {
    expect(speakClip("sit", "tip-welcome")).toBe("wave");
    expect(speakClip("sit", "hello")).toBe("wave");
    expect(speakClip("sit", "tip-calm")).toBe("talk");
  });
});
