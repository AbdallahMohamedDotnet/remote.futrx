import assert from "node:assert/strict";
import test from "node:test";
import type { ChatEvent } from "../../models/chat.ts";
import { buildChatMessageBlocks } from "./messageBlocks.ts";
import { addUsageFromEvent, EMPTY_USAGE_TOTALS } from "./usage.ts";

test("projects chat events into the existing message and usage model", () => {
  const events: ChatEvent[] = [
    { type: "user", text: "hello", t: 1 },
    { type: "assistant_text", text: "hel", t: 2 },
    { type: "assistant_text", text: "lo", t: 3 },
    { type: "tool_use_start", id: "tool-1", name: "shell", input: { command: "pwd" }, t: 4 },
    { type: "tool_use_end", id: "tool-1", output: "/workspace", isError: false, t: 5 },
    { type: "complete", usage: { input_tokens: 3, output_tokens: 5 }, t: 6 },
  ];

  assert.deepEqual(buildChatMessageBlocks(events), [
    { type: "user", text: "hello", t: 1 },
    {
      type: "assistant",
      parts: [
        { kind: "text", text: "hello" },
        {
          kind: "tool",
          id: "tool-1",
          name: "shell",
          input: { command: "pwd" },
          output: "/workspace",
          isError: false,
          status: "done",
        },
      ],
      t: 2,
      isComplete: true,
    },
  ]);

  assert.deepEqual(events.reduce(addUsageFromEvent, EMPTY_USAGE_TOTALS), {
    inputTokens: 3,
    outputTokens: 5,
    cacheReadTokens: 0,
    cacheWriteTokens: 0,
  });
});
