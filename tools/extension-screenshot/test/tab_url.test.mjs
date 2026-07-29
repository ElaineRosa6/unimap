import assert from "node:assert/strict";
import test from "node:test";

import { resolveTabFinalURL } from "../src/tab_url.js";

test("resolveTabFinalURL retries a transient tabs.get failure", async () => {
  let calls = 0;
  const finalURL = await resolveTabFinalURL(42, {
    tabs: {
      async get() {
        calls += 1;
        if (calls === 1) {
          throw new Error("navigation race");
        }
        return { url: "https://quake.360.net/quake/#/searchResult" };
      }
    },
    scripting: {
      async executeScript() {
        return [{ result: "" }];
      }
    },
    retries: 2,
    retryDelayMs: 0
  });

  assert.equal(finalURL, "https://quake.360.net/quake/#/searchResult");
  assert.equal(calls, 2);
});

test("resolveTabFinalURL falls back to the page location", async () => {
  const finalURL = await resolveTabFinalURL(42, {
    tabs: {
      async get() {
        return { url: "" };
      }
    },
    scripting: {
      async executeScript() {
        return [{ result: "https://quake.360.net/quake/#/index" }];
      }
    },
    retries: 1,
    retryDelayMs: 0
  });

  assert.equal(finalURL, "https://quake.360.net/quake/#/index");
});

test("resolveTabFinalURL fails closed when no final URL is observable", async () => {
  await assert.rejects(
    resolveTabFinalURL(42, {
      tabs: {
        async get() {
          throw new Error("tab unavailable");
        }
      },
      scripting: {
        async executeScript() {
          throw new Error("page unavailable");
        }
      },
      retries: 2,
      retryDelayMs: 0
    }),
    /plugin_final_url_unavailable/
  );
});
