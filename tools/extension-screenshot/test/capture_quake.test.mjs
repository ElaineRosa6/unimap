import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { parseHTML } from "linkedom";

import { extractEngineAssets } from "../src/capture.js";

const root = path.resolve(import.meta.dirname, "..");

test("Quake extension DOM extraction returns structured assets", async () => {
  const html = await fs.readFile(path.join(root, "test", "fixtures", "quake-results.html"), "utf8");
  const { document, window } = parseHTML(html);
  const prev = { document: globalThis.document, window: globalThis.window, chrome: globalThis.chrome };
  globalThis.document = document;
  globalThis.window = window;
  globalThis.chrome = {
    tabs: {
      get: async () => ({ url: "https://quake.360.net/quake/#/searchResult" }),
    },
    scripting: {
      executeScript: async ({ func, args }) => [{ result: func(...(args || [])) }],
    },
  };
  try {
    const result = await extractEngineAssets(1);
    assert.equal(result.engine, "quake");
    assert.equal(result.is_login_wall, false);
    assert.equal(result.items.length, 1);
    assert.equal(result.items[0].ip, "203.0.113.10");
    assert.equal(result.items[0].port, 443);
    assert.equal(result.items[0].host, "example.test");
    assert.ok(String(result.row_selector_used).includes("item-container"), result.row_selector_used);
  } finally {
    globalThis.document = prev.document;
    globalThis.window = prev.window;
    globalThis.chrome = prev.chrome;
  }
});
