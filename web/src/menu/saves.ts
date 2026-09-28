// The save menu's pure helpers (ADR-0025).

import type { SaveView } from "../api.ts";

export function playedAgo(at: string | undefined, now: number): string {
  if (!at) return "";
  const min = Math.max(0, Math.round((now - new Date(at).getTime()) / 60000));
  if (min < 1) return "agora há pouco";
  if (min < 60) return `há ${min} min`;
  const h = Math.round(min / 60);
  if (h < 24) return `há ${h} h`;
  const d = Math.round(h / 24);
  return d === 1 ? "ontem" : `há ${d} dias`;
}

/** The most recently played save: what "Continuar" opens. */
export function lastPlayed(saves: SaveView[]): SaveView | undefined {
  return saves
    .filter((s) => !s.empty)
    .sort((a, b) => (b.played_at ?? "").localeCompare(a.played_at ?? ""))[0];
}
