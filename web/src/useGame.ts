import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, api, type GameState } from "./api.ts";

export interface GameHandle {
  state: GameState | null;
  error: string | null; // the last failed request, for the player
  unauthorized: boolean;
  refresh: () => Promise<void>;
  /** act runs an action, reports its error and refreshes the state. */
  act: <T>(f: () => Promise<T>) => Promise<T | undefined>;
  clearError: () => void;
}

/** useGame polls /api/state: fast while a job runs, slowly otherwise. */
export function useGame(): GameHandle {
  const [state, setState] = useState<GameState | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [unauthorized, setUnauthorized] = useState(false);
  const busy = useRef(false);

  const refresh = useCallback(async () => {
    try {
      setState(await api<GameState>("GET", "/api/state"));
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) setUnauthorized(true);
      else setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  const act = useCallback(
    async <T>(f: () => Promise<T>) => {
      if (busy.current) return undefined;
      busy.current = true;
      try {
        return await f();
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
        return undefined;
      } finally {
        busy.current = false;
        await refresh();
      }
    },
    [refresh],
  );

  const running = state?.job?.running ?? false;
  useEffect(() => {
    void refresh();
    const id = setInterval(() => void refresh(), running ? 1200 : 4000);
    return () => clearInterval(id);
  }, [refresh, running]);

  const clearError = useCallback(() => setError(null), []);
  return { state, error, unauthorized, refresh, act, clearError };
}
