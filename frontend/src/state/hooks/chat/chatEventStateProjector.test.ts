import assert from "node:assert/strict";
import test from "node:test";
import type { ChatEvent } from "../../../models/chat.ts";
import { chatEventStateProjector } from "./chatEventStateProjector.ts";

test("projects chat events into the existing message and usage model", () => {
  const events: ChatEvent[] = [
    { type: "user", text: "hello", t: 1 },
    { type: "assistant_text", text: "hel", t: 2 },
    { type: "assistant_text", text: "lo", t: 3 },
    { type: "tool_use_start", id: "tool-1", name: "shell", input: { command: "pwd" }, t: 4 },
    { type: "tool_use_end", id: "tool-1", output: "/workspace", isError: false, t: 5 },
    { type: "complete", usage: { input_tokens: 3, output_tokens: 5 }, t: 6 },
  ];

  const state = chatEventStateProjector.fromEvents(events, {
    hasMore: false,
  });

  assert.deepEqual(state.blocks, [
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

  assert.deepEqual(state.usageTotals, {
    inputTokens: 3,
    outputTokens: 5,
    cacheReadTokens: 0,
    cacheWriteTokens: 0,
  });
});

test("coalesces streamed reasoning deltas into one message part", () => {
  const events: ChatEvent[] = [
    { type: "user", text: "inspect it", t: 1 },
    { type: "thinking", text: "Playwright ", t: 2 },
    { type: "thinking", text: "isn't installed. ", t: 3 },
    { type: "thinking", text: "I'll use the browser instead.", t: 4 },
    { type: "assistant_text", text: "I opened the page.", t: 5 },
    { type: "complete", t: 6 },
  ];

  const state = chatEventStateProjector.fromEvents(events, { hasMore: false });

  assert.deepEqual(state.blocks, [
    { type: "user", text: "inspect it", t: 1 },
    {
      type: "assistant",
      parts: [
        {
          kind: "thinking",
          text: "Playwright isn't installed. I'll use the browser instead.",
        },
        { kind: "text", text: "I opened the page." },
      ],
      t: 2,
      isComplete: true,
    },
  ]);
});

test("prepends an older event page before current blocks and adopts hasMore", () => {
  const latest = chatEventStateProjector.fromEvents(
    [
      { seq: 4, type: "user", text: "new question", t: 4 },
      { seq: 5, type: "assistant_text", text: "a complete long answer", t: 5 },
      { seq: 305, type: "complete", t: 305 },
    ],
    { hasMore: true, nextBefore: 4 }
  );

  const state = chatEventStateProjector.prepend(latest, {
    events: [
      { seq: 1, type: "user", text: "older question", t: 1 },
      { seq: 2, type: "assistant_text", text: "older answer", t: 2 },
      { seq: 3, type: "complete", t: 3 },
    ],
    hasMore: false,
    lastSeq: 305,
  });

  assert.deepEqual(state.blocks, [
    { type: "user", text: "older question", t: 1 },
    {
      type: "assistant",
      parts: [{ kind: "text", text: "older answer" }],
      t: 2,
      isComplete: true,
    },
    { type: "user", text: "new question", t: 4 },
    {
      type: "assistant",
      parts: [{ kind: "text", text: "a complete long answer" }],
      t: 5,
      isComplete: true,
    },
  ]);
  assert.equal(state.hasOlder, false);
});
