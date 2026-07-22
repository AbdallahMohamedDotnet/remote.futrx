import assert from "node:assert/strict";
import test from "node:test";
import { chatPreferenceState } from "./chatPreferenceState.ts";

test("preserves normalized skill identity and chat defaults", () => {
  const selected = [{ name: "Review", command: " /REVIEW ", provider: "codex" as const }];
  const duplicate = { name: "review", command: "/review", provider: "codex" as const };

  assert.equal(chatPreferenceState.includesSkill(selected, duplicate, "claude"), true);
  assert.deepEqual(chatPreferenceState.withoutSkill(selected, duplicate, "claude"), []);
  assert.deepEqual(
    chatPreferenceState.resolveMeta(
      { id: "chat", title: "Chat", createdAt: 1, lastMessageAt: 1 },
      null,
      {
        provider: "codex",
        model: "gpt-5",
        mode: "code",
        reasoningEffort: "high",
        serviceTier: "priority",
      }
    ),
    {
      id: "chat",
      title: "Chat",
      createdAt: 1,
      lastMessageAt: 1,
      provider: "codex",
      model: "gpt-5",
      mode: "code",
      reasoningEffort: "high",
      serviceTier: "priority",
    }
  );
});
