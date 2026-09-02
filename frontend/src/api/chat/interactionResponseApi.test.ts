import assert from "node:assert/strict";
import test from "node:test";
import { encodeInteractionResponse } from "./interactionResponseApi.ts";

test("encodes question answers with the App Server answer envelope", () => {
  assert.deepEqual(
    encodeInteractionResponse("item/tool/requestUserInput", {
      kind: "answer_questions",
      answers: { framework: ["Preact"] },
    }),
    { result: { answers: { framework: { answers: ["Preact"] } } } }
  );
});

test("preserves current and legacy approval decisions", () => {
  assert.deepEqual(
    encodeInteractionResponse("item/commandExecution/requestApproval", {
      kind: "approve",
      scope: "once",
    }),
    { result: { decision: "accept" } }
  );
  assert.deepEqual(
    encodeInteractionResponse("item/fileChange/requestApproval", {
      kind: "approve",
      scope: "session",
    }),
    { result: { decision: "acceptForSession" } }
  );
  assert.deepEqual(
    encodeInteractionResponse("execCommandApproval", {
      kind: "approve",
      scope: "once",
    }),
    { result: { decision: "approved" } }
  );
  assert.deepEqual(
    encodeInteractionResponse("applyPatchApproval", {
      kind: "approve",
      scope: "session",
    }),
    { result: { decision: "approved_for_session" } }
  );
  assert.deepEqual(
    encodeInteractionResponse("execCommandApproval", { kind: "deny_approval" }),
    { result: { decision: { denied: { rejection: "Denied by user" } } } }
  );
  assert.deepEqual(
    encodeInteractionResponse("item/fileChange/requestApproval", {
      kind: "deny_approval",
    }),
    { result: { decision: "decline" } }
  );
  assert.deepEqual(
    encodeInteractionResponse("item/fileChange/requestApproval", {
      kind: "cancel_approval",
    }),
    { result: { decision: "cancel" } }
  );
});

test("preserves permission and elicitation response shapes", () => {
  const permissions = { filesystem: { read: true } };
  assert.deepEqual(
    encodeInteractionResponse("item/permissions/requestApproval", {
      kind: "grant_permissions",
      permissions,
      scope: "session",
    }),
    { result: { permissions, scope: "session" } }
  );
  assert.deepEqual(
    encodeInteractionResponse("item/permissions/requestApproval", {
      kind: "deny_permissions",
    }),
    { result: { permissions: {}, scope: "turn" } }
  );
  assert.deepEqual(
    encodeInteractionResponse("mcpServer/elicitation/request", {
      kind: "accept_elicitation",
      content: { choice: "yes" },
    }),
    { result: { action: "accept", content: { choice: "yes" } } }
  );
  assert.deepEqual(
    encodeInteractionResponse("mcpServer/elicitation/request", {
      kind: "decline_elicitation",
    }),
    { result: { action: "decline" } }
  );
  assert.deepEqual(
    encodeInteractionResponse("mcpServer/elicitation/request", {
      kind: "cancel_elicitation",
    }),
    { result: { action: "cancel" } }
  );
});

test("preserves generic results and unsupported JSON-RPC errors", () => {
  assert.deepEqual(
    encodeInteractionResponse("future/request", {
      kind: "submit_provider_result",
      result: { acknowledged: true },
    }),
    { result: { acknowledged: true } }
  );
  assert.deepEqual(
    encodeInteractionResponse("future/request", { kind: "decline_unsupported" }),
    {
      error: {
        code: -32601,
        message: "Unsupported provider request declined by user",
      },
    }
  );
});
