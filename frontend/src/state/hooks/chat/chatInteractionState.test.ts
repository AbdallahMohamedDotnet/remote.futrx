import assert from "node:assert/strict";
import test from "node:test";
import {
  canSendInteractionResponse,
  dispatchQuestionAnswer,
} from "./chatInteractionState.ts";

test("allows interaction responses only on a synced streaming connection", () => {
  assert.equal(canSendInteractionResponse({
    status: "streaming",
    wsReady: true,
    synced: true,
    streamOpen: true,
  }), true);

  for (const blocked of [
    { status: "ready" as const, wsReady: true, synced: true, streamOpen: true },
    { status: "streaming" as const, wsReady: false, synced: true, streamOpen: true },
    { status: "streaming" as const, wsReady: true, synced: false, streamOpen: true },
    { status: "streaming" as const, wsReady: true, synced: true, streamOpen: false },
  ]) {
    assert.equal(canSendInteractionResponse(blocked), false);
  }
});

test("routes interactive answers by correlation and legacy answers as prompts", () => {
  const prompts: string[] = [];
  const interactions: Array<{ id: string; answers: Record<string, string[]> }> = [];
  const transport = {
    sendPrompt: (text: string) => {
      prompts.push(text);
      return true;
    },
    sendInteractionResponse: (id: string, answers: Record<string, string[]>) => {
      interactions.push({ id, answers });
      return true;
    },
  };

  assert.equal(dispatchQuestionAnswer({
    interactionId: "request-1",
    text: "Q: Environment?\nA: QA",
    preview: "Environment: QA",
    answers: { environment: ["QA"] },
  }, transport), true);
  assert.deepEqual(interactions, [{
    id: "request-1",
    answers: { environment: ["QA"] },
  }]);
  assert.deepEqual(prompts, []);

  assert.equal(dispatchQuestionAnswer({
    text: "Q: Continue?\nA: Yes",
    preview: "Continue: Yes",
    answers: {},
  }, transport), true);
  assert.deepEqual(prompts, ["Q: Continue?\nA: Yes"]);
  assert.equal(interactions.length, 1);

  assert.equal(dispatchQuestionAnswer({
    text: "Q: Token?\nA: secret",
    preview: "Token: Secret response received",
    answers: {},
    sensitive: true,
  }, transport), false);
  assert.deepEqual(prompts, ["Q: Continue?\nA: Yes"]);
});
