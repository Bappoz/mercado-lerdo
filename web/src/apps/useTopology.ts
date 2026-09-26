import { useCallback, useEffect, useState } from "react";
import { api, type TopologyView } from "../api.ts";

/**
 * useTopology reads the workspace's topology against the running one. The
 * file changes in the player's editor, so it is re-read every few seconds.
 * version changes after anything that may have changed it (a deploy, an install).
 */
export function useTopology(enabled: boolean, version: unknown) {
  const [view, setView] = useState<TopologyView | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setView(await api<TopologyView>("GET", "/api/topology"));
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  // biome-ignore lint/correctness/useExhaustiveDependencies: version only triggers a reload
  useEffect(() => {
    if (!enabled) return;
    void load();
    const id = setInterval(() => void load(), 4000);
    return () => clearInterval(id);
  }, [enabled, load, version]);

  return { view, error, reload: load };
}

export const ACTION: Record<"create" | "recreate" | "remove", string> = {
  create: "novo",
  recreate: "muda",
  remove: "sai",
};

export const COMPONENT_NAME: Record<string, string> = {
  nginx: "Nginx",
  app: "App Nexora",
  pgbouncer: "PgBouncer",
  postgres: "PostgreSQL",
};

export const usd = (v: number) => `US$ ${v.toFixed(0)}`;
