import assert from "node:assert/strict";
import test from "node:test";
import {
  interactionActivityMessage,
  interactionResponseMessage,
} from "./chatStreamMessages.ts";

test("builds a correlated structured interaction response", () => {
  assert.deepEqual(
    interactionResponseMessage("request-1", { environment: ["QA"] }),
    {
      type: "interaction_response",
      id: "request-1",
      answers: { environment: ["QA"] },
    },
  );
});

test("builds a correlated interaction activity signal", () => {
  assert.deepEqual(interactionActivityMessage("request-1"), {
    type: "interaction_activity",
    id: "request-1",
  });
});
