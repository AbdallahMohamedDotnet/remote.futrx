import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const appStyles = readFileSync(new URL("../index.css", import.meta.url), "utf8");

test("scroll containers do not opt into legacy WebKit touch compositing", () => {
  assert.doesNotMatch(appStyles, /-webkit-overflow-scrolling\s*:\s*touch/i);
});
