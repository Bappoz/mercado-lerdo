// Flash Sale: a creator mentions Nexora. Traffic ramps up fast and people
// search for what they saw in the video before buying.
import { endpointThresholds, runMix, setupSessions, summary } from './lib/traffic.js';

const PEAK = parseInt(__ENV.RATE || '60', 10);
const TIMEOUT_S = 5;
// Enough VUs for every in-flight request to wait the full client timeout.
// All of them are pre-allocated: allocating on demand lags behind latency spikes.
const MAX_VUS = Math.ceil(PEAK * TIMEOUT_S * 1.2) + 10;

export const options = {
  discardResponseBodies: true,
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max', 'count'],
  scenarios: {
    flash_sale: {
      executor: 'ramping-arrival-rate',
      startRate: Math.max(1, Math.floor(PEAK / 10)),
      timeUnit: '1s',
      preAllocatedVUs: MAX_VUS,
      maxVUs: MAX_VUS,
      stages: [
        { target: PEAK, duration: __ENV.RAMP || '15s' },
        { target: PEAK, duration: __ENV.DURATION || '45s' },
      ],
    },
  },
  thresholds: endpointThresholds(['products', 'search', 'product', 'checkout']),
};

const MIX = { browse: 30, search: 35, product: 25, checkout: 10 };

export const setup = setupSessions;
export default (data) => runMix(MIX, data.tokens);
export const handleSummary = (data) => summary(data, { maxVUs: MAX_VUS, peakRate: PEAK, clientTimeoutS: TIMEOUT_S });
