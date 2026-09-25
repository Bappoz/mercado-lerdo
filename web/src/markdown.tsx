// A small Markdown subset rendered straight to React elements: no HTML string
// is ever built, so scenario text cannot inject markup (threat model T5).
// Supported: # headings, paragraphs, **bold**, *italic*, `code`, indented and
// fenced code blocks, - and 1. lists, | tables |, ---, and http(s) [links](…).

import type { ReactNode } from "react";

export type Block =
  | { t: "h"; level: number; text: string }
  | { t: "p"; text: string }
  | { t: "code"; text: string }
  | { t: "ul" | "ol"; items: string[] }
  | { t: "table"; head: string[]; rows: string[][] }
  | { t: "hr" };

const isTableRow = (l: string) => /^\s*\|.*\|\s*$/.test(l);
const cells = (l: string) =>
  l
    .trim()
    .replace(/^\||\|$/g, "")
    .split("|")
    .map((c) => c.trim());

export function parseBlocks(md: string): Block[] {
  const lines = md.replace(/\r\n/g, "\n").split("\n");
  const out: Block[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (line.trim() === "") {
      i++;
      continue;
    }
    if (line.startsWith("```")) {
      const body: string[] = [];
      for (i++; i < lines.length && !lines[i].startsWith("```"); i++) body.push(lines[i]);
      i++; // closing fence
      out.push({ t: "code", text: body.join("\n") });
      continue;
    }
    if (/^( {4}|\t)/.test(line)) {
      const body: string[] = [];
      while (i < lines.length && (/^( {4}|\t)/.test(lines[i]) || lines[i].trim() === "")) {
        body.push(lines[i].replace(/^( {4}|\t)/, ""));
        i++;
      }
      out.push({ t: "code", text: body.join("\n").trimEnd() });
      continue;
    }
    const h = /^(#{1,4})\s+(.*)$/.exec(line);
    if (h) {
      out.push({ t: "h", level: h[1].length, text: h[2].trim() });
      i++;
      continue;
    }
    if (/^\s*(-{3,}|\*{3,})\s*$/.test(line)) {
      out.push({ t: "hr" });
      i++;
      continue;
    }
    if (isTableRow(line) && i + 1 < lines.length && /^\s*\|[\s:|-]+\|\s*$/.test(lines[i + 1])) {
      const head = cells(line);
      const rows: string[][] = [];
      for (i += 2; i < lines.length && isTableRow(lines[i]); i++) rows.push(cells(lines[i]));
      out.push({ t: "table", head, rows });
      continue;
    }
    const list = /^\s*([-*]|\d+\.)\s+/.exec(line);
    if (list) {
      const ordered = /\d/.test(list[1]);
      const items: string[] = [];
      while (i < lines.length) {
        const m = /^\s*([-*]|\d+\.)\s+(.*)$/.exec(lines[i]);
        if (m) {
          items.push(m[2]);
        } else if (lines[i].trim() !== "" && /^\s{2,}/.test(lines[i]) && items.length > 0) {
          items[items.length - 1] += ` ${lines[i].trim()}`; // continuation line
        } else {
          break;
        }
        i++;
      }
      out.push({ t: ordered ? "ol" : "ul", items });
      continue;
    }
    const para: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() !== "" &&
      !/^(#{1,4}\s|```|( {4}|\t)|\s*([-*]|\d+\.)\s|\s*-{3,}\s*$)/.test(lines[i]) &&
      !isTableRow(lines[i])
    ) {
      para.push(lines[i].trim());
      i++;
    }
    out.push({ t: "p", text: para.join(" ") });
  }
  return out;
}

export type Inline =
  | { t: "text"; v: string }
  | { t: "b" | "i" | "code"; v: string }
  | { t: "a"; v: string; href: string };

const INLINE = /(`[^`]+`)|(\*\*[^*]+\*\*)|(\*[^*\s][^*]*\*)|(\[[^\]]+\]\([^)\s]+\))/g;

export function parseInline(s: string): Inline[] {
  const out: Inline[] = [];
  let last = 0;
  for (const m of s.matchAll(INLINE)) {
    const at = m.index ?? 0;
    if (at > last) out.push({ t: "text", v: s.slice(last, at) });
    const tok = m[0];
    if (m[1]) out.push({ t: "code", v: tok.slice(1, -1) });
    else if (m[2]) out.push({ t: "b", v: tok.slice(2, -2) });
    else if (m[3]) out.push({ t: "i", v: tok.slice(1, -1) });
    else {
      const [, text, href] = /^\[([^\]]+)\]\(([^)\s]+)\)$/.exec(tok) ?? [];
      // Only web links; anything else (javascript:, data:…) stays plain text.
      if (/^https?:\/\//i.test(href)) out.push({ t: "a", v: text, href });
      else out.push({ t: "text", v: tok });
    }
    last = at + tok.length;
  }
  if (last < s.length) out.push({ t: "text", v: s.slice(last) });
  return out;
}

function renderInline(s: string): ReactNode[] {
  return parseInline(s).map((n, k) => {
    switch (n.t) {
      case "b":
        // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
        return <strong key={k}>{n.v}</strong>;
      case "i":
        // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
        return <em key={k}>{n.v}</em>;
      case "code":
        // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
        return <code key={k}>{n.v}</code>;
      case "a":
        return (
          // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
          <a key={k} href={n.href} target="_blank" rel="noopener noreferrer">
            {n.v}
          </a>
        );
      default:
        return n.v;
    }
  });
}

export function Markdown({ source }: { source: string }) {
  return (
    <div className="md">
      {parseBlocks(source).map((b, k) => {
        switch (b.t) {
          case "h": {
            const H = `h${Math.min(b.level + 1, 5)}` as "h2" | "h3" | "h4" | "h5";
            // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
            return <H key={k}>{renderInline(b.text)}</H>;
          }
          case "p":
            // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
            return <p key={k}>{renderInline(b.text)}</p>;
          case "code":
            return (
              // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
              <pre key={k}>
                <code>{b.text}</code>
              </pre>
            );
          case "ul":
          case "ol": {
            const L = b.t;
            return (
              // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
              <L key={k}>
                {b.items.map((it, j) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
                  <li key={j}>{renderInline(it)}</li>
                ))}
              </L>
            );
          }
          case "table":
            return (
              // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
              <div className="md-table" key={k}>
                <table>
                  <thead>
                    <tr>
                      {b.head.map((c, j) => (
                        // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
                        <th key={j}>{renderInline(c)}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {b.rows.map((r, j) => (
                      // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
                      <tr key={j}>
                        {r.map((c, x) => (
                          // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
                          <td key={x}>{renderInline(c)}</td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            );
          case "hr":
            // biome-ignore lint/suspicious/noArrayIndexKey: static content, never reordered
            return <hr key={k} />;
          default:
            return null;
        }
      })}
    </div>
  );
}
