import { describe, expect, it } from "vitest";
import { parseBlocks, parseInline } from "./markdown.tsx";

describe("parseBlocks", () => {
  it("reads the constructs the scenario texts use", () => {
    const md = [
      "# Title",
      "",
      "One line",
      "and its continuation.",
      "",
      "    /home/p/workspace",
      "",
      "| Tool | For |",
      "|---|---|",
      "| **Pulse** | signals |",
      "",
      "- a",
      "- b",
      "  still b",
      "",
      "1. first",
      "2. second",
      "",
      "---",
      "```",
      "EXPLAIN ANALYZE",
      "```",
    ].join("\n");
    expect(parseBlocks(md)).toEqual([
      { t: "h", level: 1, text: "Title" },
      { t: "p", text: "One line and its continuation." },
      { t: "code", text: "/home/p/workspace" },
      { t: "table", head: ["Tool", "For"], rows: [["**Pulse**", "signals"]] },
      { t: "ul", items: ["a", "b still b"] },
      { t: "ol", items: ["first", "second"] },
      { t: "hr" },
      { t: "code", text: "EXPLAIN ANALYZE" },
    ]);
  });

  it("treats HTML as text", () => {
    expect(parseBlocks("<img src=x onerror=alert(1)>")).toEqual([
      { t: "p", text: "<img src=x onerror=alert(1)>" },
    ]);
  });
});

describe("parseInline", () => {
  it("splits bold, italic, code and links", () => {
    expect(parseInline("a **b** *c* `d` [e](https://x.dev)")).toEqual([
      { t: "text", v: "a " },
      { t: "b", v: "b" },
      { t: "text", v: " " },
      { t: "i", v: "c" },
      { t: "text", v: " " },
      { t: "code", v: "d" },
      { t: "text", v: " " },
      { t: "a", v: "e", href: "https://x.dev" },
    ]);
  });

  it("keeps non-web links as plain text", () => {
    expect(parseInline("[x](javascript:alert(1))")).toEqual([
      { t: "text", v: "[x](javascript:alert(1)" },
      { t: "text", v: ")" },
    ]);
    expect(parseInline("[x](data:text/html,hi)")).toEqual([
      { t: "text", v: "[x](data:text/html,hi)" },
    ]);
  });

  it("does not read a lone asterisk as italic", () => {
    expect(parseInline("2 * 3 = 6")).toEqual([{ t: "text", v: "2 * 3 = 6" }]);
  });
});
