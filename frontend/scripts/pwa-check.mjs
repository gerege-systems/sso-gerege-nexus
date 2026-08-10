#!/usr/bin/env node
/**
 * Holds the one rule the service worker exists to keep.
 *
 *   npm run pwa:check
 *
 * A service worker sits in front of every request the app makes, and this
 * platform's requests carry tenant data. The worker is written so that nothing
 * belonging to a person is ever stored: /api is not read from the cache and not
 * written to it, and only build-addressed assets are kept.
 *
 * That rule lives in a handful of early returns, which is exactly the kind of
 * code a later change walks through without noticing. The failure would be
 * silent and would look like a performance improvement: two people sign in to
 * the same browser an hour apart, and the second is handed the first one's
 * documents by a worker that knows nothing about sessions.
 *
 * So the worker's decisions are asserted rather than trusted. The real file is
 * loaded with stubbed service-worker globals and asked, for each shape of
 * request, whether it takes the request over or lets it through.
 *
 * Exits non-zero on any wrong answer.
 */
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import vm from "node:vm";

const here = dirname(fileURLToPath(import.meta.url));
const workerPath = join(here, "..", "public", "sw.js");
const ORIGIN = "https://nexus.gerege.mn";

const listeners = {};
const sandbox = {
  self: {
    addEventListener: (name, fn) => (listeners[name] = fn),
    skipWaiting: async () => {},
    clients: { claim: async () => {} },
    location: { origin: ORIGIN },
  },
  // Enough of the Cache API to load; the assertions are about routing, not
  // about what ends up stored.
  caches: {
    open: async () => ({ addAll: async () => {}, put: async () => {}, match: async () => undefined }),
    keys: async () => [],
    delete: async () => true,
    match: async () => undefined,
  },
  fetch: async () => ({ ok: true, type: "basic", clone: () => ({}) }),
  Response: { error: () => ({}) },
  URL,
  Promise,
  console,
};

vm.createContext(sandbox);
vm.runInContext(readFileSync(workerPath, "utf8"), sandbox);

if (typeof listeners.fetch !== "function") {
  console.error("sw.js registered no fetch handler — nothing to check");
  process.exit(1);
}

/** Whether the worker took this request over. */
function handles(url, { mode = "no-cors", method = "GET", cache = "default" } = {}) {
  let taken = false;
  listeners.fetch({
    request: { url, method, mode, cache },
    respondWith: () => {
      taken = true;
    },
  });
  return taken;
}

const cases = [
  // The rule. Every shape an API call can arrive in has to pass through.
  ["an API read", `${ORIGIN}/api/v1/contacts/`, {}, false],
  ["an API read with a query", `${ORIGIN}/api/v1/documents?tenant=x`, {}, false],
  ["an API call shaped like a navigation", `${ORIGIN}/api/v1/auth/me`, { mode: "navigate" }, false],
  ["a write of any kind", `${ORIGIN}/contacts`, { method: "POST" }, false],
  ["somebody else's origin", "https://sso.gerege.mn/x", {}, false],
  // Serving either of these from storage is how an install gets stuck on a
  // version nobody is running.
  ["the worker itself", `${ORIGIN}/sw.js`, {}, false],
  ["the manifest", `${ORIGIN}/manifest.webmanifest`, {}, false],
  // A caller that opted out of the HTTP cache is asking for the network.
  ["a no-store request", `${ORIGIN}/_next/static/chunks/a.js`, { cache: "no-store" }, false],

  // What the worker is for.
  ["a build asset", `${ORIGIN}/_next/static/chunks/a.js`, {}, true],
  ["an app icon", `${ORIGIN}/icons/app-192.png`, {}, true],
  ["the brand mark", `${ORIGIN}/brand.webp`, {}, true],
  ["a page navigation", `${ORIGIN}/documents`, { mode: "navigate" }, true],
];

let wrong = 0;
for (const [name, url, options, expected] of cases) {
  const actual = handles(url, options);
  if (actual !== expected) {
    wrong += 1;
    console.error(
      `  ${name}: the worker ${actual ? "took it over" : "let it through"}, expected the opposite`,
    );
  }
}

if (wrong > 0) {
  console.error(`\n${wrong} service-worker routing decision(s) wrong — see public/sw.js`);
  process.exit(1);
}
console.log(`service worker: ${cases.length} routing decisions correct`);
