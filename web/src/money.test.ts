import { describe, expect, it } from "vitest";
import { money } from "./money.ts";

describe("money", () => {
  it("matches the server's format", () => {
    expect(money(0)).toBe("R$ 0");
    expect(money(1500)).toBe("R$ 1.500");
    expect(money(1234567)).toBe("R$ 1.234.567");
    expect(money(-150)).toBe("-R$ 150");
  });
});
