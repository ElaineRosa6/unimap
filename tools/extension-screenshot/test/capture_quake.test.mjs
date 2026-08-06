import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import puppeteer from "puppeteer";

const root = path.resolve(import.meta.dirname, "..");
const captureSource = await fs.readFile(path.join(root, "src", "capture.js"), "utf8");
const captureModule = await import(`data:text/javascript;base64,${Buffer.from(captureSource).toString("base64")}`);

test("Quake extension DOM extraction returns structured assets", async (t) => {
  const executablePath = process.env.UNIMAP_CHROME_PATH;
  assert.ok(executablePath, "UNIMAP_CHROME_PATH is required");
  const browser = await puppeteer.launch({
    executablePath,
    headless: true,
    args: ["--no-sandbox", "--disable-gpu"],
  });
  t.after(() => browser.close());

  const page = await browser.newPage();
  const fixturePath = path.join(root, "test", "fixtures", "quake-results.html");
  await page.goto(pathToFileURL(fixturePath).href);

  globalThis.chrome = {
    tabs: {
      get: async () => ({ url: "https://quake.360.net/quake/#/searchResult" }),
    },
    scripting: {
      executeScript: async ({ func, args }) => [{
        result: await page.evaluate(func, ...args),
      }],
    },
  };

  const result = await captureModule.extractEngineAssets(1);
  assert.equal(result.engine, "quake");
  assert.equal(result.items.length, 1);
  assert.equal(result.items[0].ip, "203.0.113.10");
  assert.equal(result.items[0].port, 443);
  assert.equal(result.items[0].host, "example.test");
  assert.equal(result.row_selector_used, ".item-container");
});
