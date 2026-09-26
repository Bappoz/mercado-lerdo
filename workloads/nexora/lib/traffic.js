// Shared traffic model for Nexora workloads.
//
// Determinism: every decision (journey, search term, product) derives from
// (SEED, global iteration number), not from Math.random or the VU that runs it.
// The same seed therefore replays the same sequence of user intents; only the
// timing varies between runs.
import { sleep } from "k6";
import http from "k6/http";
import exec from "k6/execution";

export const BASE_URL = __ENV.BASE_URL || "http://nginx:8080";
export const SEED = parseInt(__ENV.SEED || "4242", 10);
export const RUN_ID = __ENV.RUN_ID || `run${Date.now()}`;
export const PRODUCTS = parseInt(__ENV.PRODUCTS || "100000", 10);
// Users abandon slow requests. This also bounds the VUs an open-model test
// needs (≈ rate × timeout), so dropped_iterations reflects generator limits,
// not a stuck system.
export const TIMEOUT = __ENV.CLIENT_TIMEOUT || "5s";
const SESSIONS = parseInt(__ENV.SESSIONS || "20", 10);

// --- deterministic hashing -------------------------------------------------

// mix32 is a 32-bit integer hash (murmur3 finalizer).
function mix32(h) {
  h = Math.imul(h ^ (h >>> 16), 0x85ebca6b);
  h = Math.imul(h ^ (h >>> 13), 0xc2b2ae35);
  return (h ^ (h >>> 16)) >>> 0;
}

// rand returns a deterministic float in [0, 1) for (iteration, salt).
export function rand(iteration, salt) {
  return mix32(mix32(SEED ^ Math.imul(salt, 0x9e3779b1)) ^ iteration) / 4294967296;
}

// --- search terms: Zipf-like long tail ---------------------------------------

// Words that exist in product names (see internal/seed/words.go), plus terms
// that match nothing — real users misspell and search for things you don't sell.
const MATCHING = [
  "lamp",
  "chair",
  "mug",
  "kettle",
  "backpack",
  "speaker",
  "headphones",
  "mat",
  "desk",
  "tent",
  "lantern",
  "bottle",
  "oak",
  "steel",
  "bamboo",
  "ceramic",
  "modern",
  "vintage",
  "compact",
  "premium",
  "rug",
  "knife",
  "pan",
  "planter",
  "walnut",
  "linen",
  "copper",
  "leather",
  "nordic",
  "portable",
  "smart",
  "retro",
  "suitcase",
  "hammock",
  "grill",
  "dumbbell",
  "kettlebell",
  "earbuds",
  "mirror",
  "clock",
  "vase",
  "teapot",
  "notebook",
  "stapler",
  "turntable",
  "rake",
  "hose",
];
const NO_MATCH = ["iphone", "laptop", "sneakers", "lamb", "chiar", "kettel", "drone", "tv"];
const TERMS = MATCHING.concat(NO_MATCH);
const ZIPF_S = 1.1;
const CUMULATIVE = (() => {
  const w = TERMS.map((_, i) => 1 / Math.pow(i + 1, ZIPF_S));
  const total = w.reduce((a, b) => a + b, 0);
  let acc = 0;
  return w.map((x) => (acc += x / total));
})();

export function searchTerm(iteration) {
  const r = rand(iteration, 1);
  const i = CUMULATIVE.findIndex((c) => r < c);
  return TERMS[i === -1 ? TERMS.length - 1 : i];
}

export function productId(iteration, salt) {
  return 1 + Math.floor(rand(iteration, salt) * PRODUCTS);
}

// --- journeys --------------------------------------------------------------

const params = (name, extra = {}) => ({ timeout: TIMEOUT, tags: { name }, ...extra });

export function browse(it) {
  http.get(`${BASE_URL}/products?page=${1 + Math.floor(rand(it, 2) * 50)}`, params("products"));
}

export function search(it) {
  http.get(`${BASE_URL}/search?q=${encodeURIComponent(searchTerm(it))}`, params("search"));
}

export function viewProduct(it) {
  http.get(`${BASE_URL}/products/${productId(it, 3)}`, params("product"));
}

// checkout places one order. With retry, a customer whose checkout failed
// (server error, connection error or timeout) presses the button again a
// second later, with the same order and Idempotency-Key, as real ones do.
export function checkout(it, tokens, retry = false) {
  const token = tokens[it % tokens.length];
  const ref = `${RUN_ID}-${it}`;
  const send = () =>
    http.post(
      `${BASE_URL}/checkout`,
      JSON.stringify({ order_ref: ref, items: [{ product_id: productId(it, 4), quantity: 1 }] }),
      params("checkout", {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
          "Idempotency-Key": ref,
        },
      }),
    );
  const res = send();
  if (retry && (res.status === 0 || res.status >= 500)) {
    sleep(1);
    send();
  }
}

// runMix picks one journey per iteration according to mix ({name: weight}).
// opts.retry makes failed checkouts retry once (see checkout).
export function runMix(mix, tokens, opts = {}) {
  const it = exec.scenario.iterationInTest;
  const total = Object.values(mix).reduce((a, b) => a + b, 0);
  let r = rand(it, 0) * total;
  for (const [name, weight] of Object.entries(mix)) {
    r -= weight;
    if (r < 0) {
      JOURNEYS[name](it, tokens, opts.retry);
      return;
    }
  }
}

const JOURNEYS = { browse, search, product: viewProduct, checkout };

// setupSessions logs in the seeded demo users once, before the load starts, so
// the (deliberately expensive) password hashing does not dominate the test.
export function setupSessions() {
  const tokens = [];
  for (let i = 1; i <= SESSIONS; i++) {
    const email = `user${String(i).padStart(5, "0")}@nexora.test`;
    const res = http.post(
      `${BASE_URL}/auth/login`,
      JSON.stringify({ email, password: __ENV.USER_PASSWORD || "nexora-demo" }),
      // responseType overrides discardResponseBodies: we need the token.
      {
        headers: { "Content-Type": "application/json" },
        tags: { name: "setup_login" },
        responseType: "text",
      },
    );
    if (res.status !== 200) {
      throw new Error(`setup: login ${email} returned ${res.status}`);
    }
    tokens.push(res.json("token"));
  }
  return { tokens };
}

// Referencing each endpoint in thresholds makes k6 report per-endpoint
// sub-metrics in the summary. The thresholds themselves always pass: pass/fail
// is the evaluator's job, not the load generator's.
export function endpointThresholds(names) {
  const t = {};
  for (const n of names) {
    t[`http_req_duration{name:${n}}`] = ["max>=0"];
    t[`http_req_failed{name:${n}}`] = ["rate>=0"];
    t[`http_reqs{name:${n}}`] = ["count>=0"];
  }
  return t;
}

// summary prints the k6 summary as JSON on stdout for `sesim load-summary`.
// meta ({maxVUs, peakRate, clientTimeoutS}) lets the reader tell a saturated
// system (every VU stuck waiting) from a generator that could not keep up.
// setup_data holds live session tokens: never write it out.
export function summary(data, meta) {
  delete data.setup_data;
  data.sesim = meta;
  return { stdout: JSON.stringify(data) };
}
