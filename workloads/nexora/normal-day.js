// Normal Day: steady traffic, mostly browsing. The system should be healthy.
import { endpointThresholds, runMix, setupSessions, summary } from './lib/traffic.js';

const RATE = parseInt(__ENV.RATE || '10', 10);
const TIMEOUT_S = 5;
// Enough VUs for every in-flight request to wait the full client timeout.
// All of them are pre-allocated: allocating on demand lags behind latency spikes.
const MAX_VUS = Math.ceil(RATE * TIMEOUT_S * 1.2) + 10;

export const options = {
  discardResponseBodies: true,
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max', 'count'],
  scenarios: {
    normal_day: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: __ENV.DURATION || '60s',
      preAllocatedVUs: MAX_VUS,
      maxVUs: MAX_VUS,
    },
  },
  thresholds: endpointThresholds(['products', 'search', 'product', 'checkout']),
};

const MIX = { browse: 45, search: 15, product: 35, checkout: 5 };

export const setup = setupSessions;
export default (data) => runMix(MIX, data.tokens);
export const handleSummary = (data) => summary(data, { maxVUs: MAX_VUS, peakRate: RATE, clientTimeoutS: TIMEOUT_S });
