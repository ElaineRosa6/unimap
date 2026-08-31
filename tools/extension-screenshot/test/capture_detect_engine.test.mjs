import assert from "node:assert/strict";
import test from "node:test";

import { detectEngine } from "../src/capture.js";

const cases = [
  ["https://fofa.info/result?qbase64=cG9ydD0iODAi", "fofa"],
  ["https://hunter.qianxin.com/home/list?search=abc", "hunter"],
  ["https://www.zoomeye.org/searchResult?q=port%3A80", "zoomeye"],
  ["https://www.zoomeye.ai/search?q=port%3A80", "zoomeye"],
  ["https://quake.360.net/quake/#/searchResult?searchVal=port%3A80", "quake"],
  ["https://www.shodan.io/search?query=port%3A80", "shodan"],
  ["https://search.shodan.io/search?query=apache", "shodan"],
  ["https://platform.censys.io/search?resource=hosts&q=services.port%3A443", "censys"],
  ["https://search.censys.io/hosts?q=services.port%3A80", "censys"],
  ["https://www.daydaymap.com/searchResult?keyword=port%3D443", "daydaymap"],
  ["https://example.com/search", "unknown"],
];

for (const [url, engine] of cases) {
  test(`detectEngine maps ${url} to ${engine}`, () => {
    assert.equal(detectEngine(url), engine);
  });
}
