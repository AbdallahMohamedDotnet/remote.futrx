import assert from "node:assert/strict";
import test from "node:test";
import {
  autoResolutionRemainingSeconds,
  formatAutoResolutionRemaining,
} from "./autoResolutionCountdown.ts";

test("hides the first minute and shows the final minute of Codex auto-resolution", () => {
  const startedAt = 1_000_000;

  assert.equal(autoResolutionRemainingSeconds(startedAt, startedAt), null);
  assert.equal(autoResolutionRemainingSeconds(startedAt, startedAt + 59_999), null);
  assert.equal(autoResolutionRemainingSeconds(startedAt, startedAt + 60_000), 60);
  assert.equal(autoResolutionRemainingSeconds(startedAt, startedAt + 60_001), 60);
  assert.equal(autoResolutionRemainingSeconds(startedAt, startedAt + 119_001), 1);
  assert.equal(autoResolutionRemainingSeconds(startedAt, startedAt + 120_000), null);
});

test("formats the pinned Codex countdown convention", () => {
  assert.equal(formatAutoResolutionRemaining(60), "1m 00s");
  assert.equal(formatAutoResolutionRemaining(59), "59s");
  assert.equal(formatAutoResolutionRemaining(1), "1s");
});
