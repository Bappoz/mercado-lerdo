import { describe, expect, it } from "vitest";
import type { SaveView } from "../api.ts";
import { lastPlayed, playedAgo } from "./saves.ts";

const save = (slot: number, over: Partial<SaveView>): SaveView => ({
  slot,
  current: false,
  empty: false,
  missions: 0,
  balance: 0,
  ...over,
});

describe("menu", () => {
  it("continues the save played last, never an empty one", () => {
    const saves = [
      save(1, { played_at: "2026-09-27T10:00:00Z" }),
      save(2, { empty: true }),
      save(3, { played_at: "2026-09-28T09:00:00Z" }),
    ];
    expect(lastPlayed(saves)?.slot).toBe(3);
    expect(lastPlayed([save(1, { empty: true })])).toBeUndefined();
  });

  it("says how long ago a save was played", () => {
    const now = Date.parse("2026-09-28T12:00:00Z");
    expect(playedAgo("2026-09-28T11:59:40Z", now)).toBe("agora há pouco");
    expect(playedAgo("2026-09-28T11:15:00Z", now)).toBe("há 45 min");
    expect(playedAgo("2026-09-28T07:00:00Z", now)).toBe("há 5 h");
    expect(playedAgo("2026-09-27T10:00:00Z", now)).toBe("ontem");
    expect(playedAgo("2026-09-24T12:00:00Z", now)).toBe("há 4 dias");
  });
});
