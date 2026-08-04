import assert from "node:assert/strict";
import test from "node:test";

import { readBrowserCredentials } from "../src/credentials.js";

test("readBrowserCredentials returns cookies and same-origin storage", async () => {
  const listeners = new Set();
  const removed = [];
  const chromeApi = {
    tabs: {
      query: async () => [],
      create: async ({ url, active }) => ({ id: 7, url, active }),
      get: async () => ({ status: "complete", url: "https://www.daydaymap.com/home" }),
      remove: async (id) => { removed.push(id); },
      onUpdated: { addListener: (fn) => listeners.add(fn), removeListener: (fn) => listeners.delete(fn) }
    },
    cookies: { getAll: async ({ domain }) => [{ name: "sid", value: domain }] },
    scripting: { executeScript: async () => [{ result: { local: { token: "value" }, session: { nonce: "n" } } }] }
  };

  const result = await readBrowserCredentials("https://www.daydaymap.com/home", chromeApi);
  assert.equal(result.cookies[0].value, "www.daydaymap.com");
  assert.deepEqual(result.storage, { local: { token: "value" }, session: { nonce: "n" } });
  assert.equal(result.finalURL, "https://www.daydaymap.com/home");
  assert.deepEqual(removed, [7]);
});

test("readBrowserCredentials reuses a logged-in same-origin tab", async () => {
  let createCalls = 0;
  let removeCalls = 0;
  const chromeApi = {
    tabs: {
      query: async () => [{ id: 9, active: true, status: "complete", url: "https://www.daydaymap.com/home" }],
      create: async () => { createCalls++; return { id: 10 }; },
      get: async () => ({ status: "complete", url: "https://www.daydaymap.com/home" }),
      remove: async () => { removeCalls++; },
      onUpdated: { addListener: () => {}, removeListener: () => {} }
    },
    cookies: { getAll: async () => [] },
    scripting: { executeScript: async () => [{ result: { local: { auth: "fixture" }, session: {} } }] }
  };

  const result = await readBrowserCredentials("https://www.daydaymap.com/", chromeApi);
  assert.equal(createCalls, 0);
  assert.equal(removeCalls, 0);
  assert.equal(result.storage.local.auth, "fixture");
});
