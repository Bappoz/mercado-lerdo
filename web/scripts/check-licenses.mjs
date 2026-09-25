// Fails when a production dependency uses a license outside the ADR-0009 allowlist.
// Uses `npm query` (built into npm) to avoid pulling a license-checker dependency.
import { execFileSync } from "node:child_process";

const ALLOWED = new Set([
  "MIT",
  "ISC",
  "BSD-2-Clause",
  "BSD-3-Clause",
  "Apache-2.0",
  "0BSD",
  "BlueOak-1.0.0",
  "CC0-1.0",
  "Unlicense",
  "MPL-2.0",
  "EPL-2.0",
]);

const deps = JSON.parse(execFileSync("npm", ["query", ".prod"], { encoding: "utf8" }));

// SPDX expressions like "(MIT OR Apache-2.0)" pass when any alternative is allowed.
const isAllowed = (license) =>
  typeof license === "string" &&
  license
    .replace(/[()]/g, "")
    .split(/\s+OR\s+/)
    .some((id) => ALLOWED.has(id.trim()));

// The root package (location "") is our own proprietary code.
const third = deps.filter((d) => d.location);

const violations = third
  .filter((d) => !isAllowed(d.license))
  .map((d) => `${d.name}@${d.version}: ${JSON.stringify(d.license ?? null)}`);

if (violations.length > 0) {
  console.error(`License policy violation (ADR-0009):\n  ${violations.join("\n  ")}`);
  process.exit(1);
}
console.log(`licenses ok: ${third.length} production dependencies checked`);
