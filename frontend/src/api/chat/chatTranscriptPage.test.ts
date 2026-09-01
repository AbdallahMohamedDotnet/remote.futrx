import assert from "node:assert/strict";
import test from "node:test";
import type { ChatTranscriptPage } from "../../models/chat.ts";
import { transcriptPageToEventPage } from "./chatTranscriptPage.ts";

test("flattens complete transcript turns without changing cursor metadata", () => {
  const page: ChatTranscriptPage = {
    turns: [
      {
        id: "turn-1",
        startSeq: 1,
        endSeq: 3,
        events: [
          { seq: 1, t: 1, type: "user", text: "older question" },
          { seq: 2, t: 2, type: "assistant_text", text: "older answer" },
          { seq: 3, t: 3, type: "complete" },
        ],
      },
      {
        id: "turn-2",
        startSeq: 4,
        endSeq: 6,
        events: [
          { seq: 4, t: 4, type: "user", text: "new question" },
          { seq: 5, t: 5, type: "assistant_text", text: "new answer" },
          { seq: 6, t: 6, type: "complete" },
        ],
      },
    ],
    nextBefore: 1,
    lastSeq: 6,
    hasMore: true,
  };

  assert.deepEqual(transcriptPageToEventPage(page), {
    events: [...page.turns[0].events, ...page.turns[1].events],
    nextBefore: 1,
    lastSeq: 6,
    hasMore: true,
  });
});
