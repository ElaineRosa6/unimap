import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import test from "node:test";

import { extractEngineAssets } from "../src/capture.js";
import { parseHTML } from "linkedom";

const root = path.resolve(import.meta.dirname, "..");

const engines = [
  {
    name: "fofa",
    fixture: "fofa-results.html",
    url: "https://fofa.info/result?qbase64=cG9ydD0iODAi",
    identity: "203.0.113.8",
  },
  {
    name: "hunter",
    fixture: "hunter-results.html",
    url: "https://hunter.qianxin.com/home/list?search=cG9ydD00NDM",
    identity: "198.51.100.20",
  },
  {
    name: "zoomeye",
    fixture: "zoomeye-results.html",
    url: "https://www.zoomeye.ai/searchResult?q=port%3A8080",
    identity: "203.0.113.40",
  },
  {
    name: "quake",
    fixture: "quake-results.html",
    url: "https://quake.360.net/quake/#/searchResult",
    identity: "203.0.113.10",
  },
  {
    name: "shodan",
    fixture: "shodan-results.html",
    url: "https://www.shodan.io/search?query=port%3A80+country%3A%22CN%22",
    identity: "2600:9000:201e:1600:1f:6b53:e640:93a1",
  },
  {
    name: "censys",
    fixture: "censys-results.html",
    url: "https://platform.censys.io/search?resource=hosts&q=services.port%3A443",
    identity: "198.51.100.77",
  },
  {
    name: "daydaymap",
    fixture: "daydaymap-results.html",
    url: "https://www.daydaymap.com/searchResult?keyword=port%3D443",
    identity: "203.0.113.90",
  },
];

async function extractFromFixture(html, url) {
  const { document, window } = parseHTML(html);
  const prev = { document: globalThis.document, window: globalThis.window, chrome: globalThis.chrome };
  globalThis.document = document;
  globalThis.window = window;
  globalThis.chrome = {
    tabs: {
      get: async () => ({ url }),
    },
    scripting: {
      executeScript: async ({ func, args }) => [{ result: func(...(args || [])) }],
    },
  };
  try {
    return await extractEngineAssets(1);
  } finally {
    globalThis.document = prev.document;
    globalThis.window = prev.window;
    globalThis.chrome = prev.chrome;
  }
}

test("extractEngineAssets returns structured items for seven engine fixtures", async () => {
  for (const engine of engines) {
    const html = await fs.readFile(path.join(root, "test", "fixtures", engine.fixture), "utf8");
    const result = await extractFromFixture(html, engine.url);
    assert.equal(result.engine, engine.name, `${engine.name} engine`);
    assert.equal(result.is_login_wall, false, `${engine.name} login wall`);
    assert.ok(result.items && result.items.length >= 1, `${engine.name} items`);
    const identity = `${result.items[0].ip || ""} ${result.items[0].host || ""}`;
    assert.ok(
      identity.includes(engine.identity),
      `${engine.name} identity ${identity} missing ${engine.identity}`,
    );
  }
});
