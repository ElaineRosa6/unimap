import assert from "node:assert/strict";
import test from "node:test";

import { prepareDayDayMapSearch } from "../src/daydaymap.js";

test("prepareDayDayMapSearch injects the native query and waits for the result route", async () => {
  let injectedQuery = "";
  let tabReads = 0;
  const finalURL = await prepareDayDayMapSearch(42, 'ip.port="443"', {
    scripting: {
      async executeScript(options) {
        if (options.args) injectedQuery = options.args[0];
        return [{ result: { ok: true } }];
      }
    },
    tabs: {
      async get() {
        tabReads += 1;
        return { url: tabReads < 2 ? "https://www.daydaymap.com/home" : "https://www.daydaymap.com/searchResult" };
      }
    }
  });

  assert.equal(injectedQuery, 'ip.port="443"');
  assert.equal(finalURL, "https://www.daydaymap.com/searchResult");
});

test("prepareDayDayMapSearch returns to home when result URL has no keyword", async () => {
  let url = "https://www.daydaymap.com/searchResult";
  const updates = [];
  const finalURL = await prepareDayDayMapSearch(7, 'port="80"', {
    scripting: {
      async executeScript(options) {
        if (!options.args && url.includes("/home")) {
          url = "https://www.daydaymap.com/searchResult?keyword=cG9ydD0iODAi";
        }
        return [{ result: { ok: true } }];
      }
    },
    tabs: {
      async get() {
        return { url };
      },
      async update(_id, info) {
        updates.push(info.url);
        url = info.url;
      }
    }
  });
  assert.equal(updates[0], "https://www.daydaymap.com/home");
  assert.ok(finalURL.includes("/searchResult"));
});
