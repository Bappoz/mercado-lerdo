// Crash Day: steady buying traffic while something breaks. Customers whose
// checkout fails press the button again (same order, same Idempotency-Key).
import { endpointThresholds, runMix, setupSessions, summary } from "./lib/traffic.js";

const PEAK = parseInt(__ENV.RATE || "50", 10);
const TIMEOUT_S = 5;
// Enough VUs for every in-flight request to wait the full client timeout, plus
// a retry. All of them are pre-allocated: allocating on demand lags behind.
const MAX_VUS = Math.ceil(PEAK * (TIMEOUT_S + 6) * 1.2) + 10;

export const options = {
  discardResponseBodies: true,
  summaryTrendStats: ["avg", "min", "med", "p(90)", "p(95)", "p(99)", "max", "count"],
  scenarios: {
    crash_day: {
      executor: "ramping-arrival-rate",
      startRate: Math.max(1, Math.floor(PEAK / 10)),
      timeUnit: "1s",
      preAllocatedVUs: MAX_VUS,
      maxVUs: MAX_VUS,
      stages: [
        { target: PEAK, duration: __ENV.RAMP || "15s" },
        { target: PEAK, duration: __ENV.DURATION || "45s" },
      ],
    },
  },
  thresholds: endpointThresholds(["products", "search", "product", "checkout"]),
};

const MIX = { browse: 30, search: 20, product: 30, checkout: 20 };

export const setup = setupSessions;
export default (data) => runMix(MIX, data.tokens, { retry: true });
export const handleSummary = (data) =>
  summary(data, { maxVUs: MAX_VUS, peakRate: PEAK, clientTimeoutS: TIMEOUT_S });
