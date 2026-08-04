import assert from "node:assert/strict";
import test from "node:test";

import { normalizeLoopbackAPIBaseURL } from "../src/bridge_url.js";

test("normalizeLoopbackAPIBaseURL accepts explicit loopback listeners", () => {
  assert.equal(normalizeLoopbackAPIBaseURL(" http://127.0.0.1:8550/ "), "http://127.0.0.1:8550");
  assert.equal(normalizeLoopbackAPIBaseURL("http://localhost:8448"), "http://localhost:8448");
});

test("normalizeLoopbackAPIBaseURL rejects non-loopback and ambiguous URLs", () => {
  for (const raw of [
    "https://127.0.0.1:8448",
    "http://example.test:8448",
    "http://127.0.0.1",
    "http://user:pass@127.0.0.1:8448",
    "http://127.0.0.1:8448/?token=value",
  ]) {
    assert.throws(() => normalizeLoopbackAPIBaseURL(raw));
  }
});
