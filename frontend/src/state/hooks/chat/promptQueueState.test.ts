import assert from "node:assert/strict";
import test from "node:test";
import type { QueuedPrompt } from "../../../models/chat.ts";
import { promptQueueState } from "./promptQueueState.ts";

const prompts: QueuedPrompt[] = [
  { id: "q1", text: "first", preferences: { provider: "claude", mode: "plan" } },
  { id: "q2", text: "second", preferences: { provider: "codex", mode: "default" } },
];

test("nextDispatch sends the queue head in an open send window", () => {
  assert.equal(promptQueueState.nextDispatch(prompts, null, "ready", true), prompts[0]);
  assert.deepEqual(promptQueueState.nextDispatch(prompts, null, "ready", true)?.preferences, {
    provider: "claude",
    mode: "plan",
  });
});

test("nextDispatch holds while the status is not ready", () => {
  assert.equal(promptQueueState.nextDispatch(prompts, null, "streaming", true), null);
  assert.equal(promptQueueState.nextDispatch(prompts, null, "loading", true), null);
  assert.equal(promptQueueState.nextDispatch(prompts, null, "error", true), null);
});

test("nextDispatch holds while the connection cannot send", () => {
  assert.equal(promptQueueState.nextDispatch(prompts, null, "ready", false), null);
});

test("nextDispatch holds while a dispatch is in flight", () => {
  assert.equal(promptQueueState.nextDispatch(prompts, "q1", "ready", true), null);
});

test("nextDispatch returns null for an empty queue", () => {
  assert.equal(promptQueueState.nextDispatch([], null, "ready", true), null);
});

test("promptsAfterOutcome removes an accepted prompt", () => {
  const next = promptQueueState.promptsAfterOutcome(prompts, {
    clientId: "q1", accepted: true, retryable: false, reason: "accepted",
  });
  assert.deepEqual(next, [prompts[1]]);
});

test("promptsAfterOutcome keeps a rejected prompt queued for retry", () => {
  const next = promptQueueState.promptsAfterOutcome(prompts, {
    clientId: "q1", accepted: false, retryable: true, reason: "busy",
  });
  assert.equal(next, prompts);
});

test("promptsAfterOutcome removes a non-retryable rejection for draft restoration", () => {
  const next = promptQueueState.promptsAfterOutcome(prompts, {
    clientId: "q1", accepted: false, retryable: false, reason: "preferences_changed",
  });
  assert.deepEqual(next, [prompts[1]]);
});

test("promptsAfterOutcome ignores an outcome for an unknown prompt", () => {
  const next = promptQueueState.promptsAfterOutcome(prompts, {
    clientId: "zz", accepted: true, retryable: false, reason: "accepted",
  });
  assert.equal(next, prompts);
});

test("inflightAfterOutcome frees the latch on a verdict for the in-flight prompt", () => {
  assert.equal(promptQueueState.inflightAfterOutcome("q1", {
    clientId: "q1", accepted: false, retryable: true, reason: "busy",
  }), null);
  assert.equal(promptQueueState.inflightAfterOutcome("q1", {
    clientId: "q1", accepted: true, retryable: false, reason: "accepted",
  }), null);
});

test("inflightAfterOutcome keeps the latch on a verdict for another prompt", () => {
  const outcome = { clientId: "q2", accepted: true, retryable: false, reason: "accepted" };
  assert.equal(promptQueueState.inflightAfterOutcome("q1", outcome), "q1");
  assert.equal(promptQueueState.inflightAfterOutcome(null, outcome), null);
});
