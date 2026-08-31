import assert from "node:assert/strict";
import test from "node:test";
import type { ChatMeta } from "../../models/chat.ts";
import { workspaceDataProjector } from "./workspaceDataProjector.ts";

test("detects generic and legacy provider session changes", () => {
  const current: ChatMeta[] = [{
    id: "chat",
    title: "Chat",
    createdAt: 1,
    lastMessageAt: 1,
    sessions: { "future-agent": "session-1" },
    kimiSessionId: "kimi-1",
    antigravitySessionId: "agy-1",
  }];

  const same = workspaceDataProjector.replaceChats([{
    ...current[0],
    sessions: { "future-agent": "session-1" },
  }], current);
  assert.equal(same, current);

  const genericChanged = workspaceDataProjector.replaceChats([{
    ...current[0],
    sessions: { "future-agent": "session-2" },
  }], current);
  assert.notEqual(genericChanged, current);

  const legacyChanged = workspaceDataProjector.replaceChats([{
    ...current[0],
    antigravitySessionId: "agy-2",
  }], current);
  assert.notEqual(legacyChanged, current);
});

test("keeps skill-only chat upserts instead of dropping them as unchanged", () => {
  const current: ChatMeta[] = [{
    id: "chat",
    title: "Chat",
    createdAt: 1,
    lastMessageAt: 1,
    selectedSkills: [{ name: "browser", command: "browser", provider: "claude", source: "builtin" }],
  }];

  const same = workspaceDataProjector.upsertChat(current, {
    ...current[0],
    selectedSkills: [{ name: "browser", command: "browser", provider: "claude", source: "builtin" }],
  });
  assert.equal(same, current);

  // The removal upsert omits selectedSkills entirely (server drops the empty
  // list), which is what left the chip on screen before.
  const cleared = workspaceDataProjector.upsertChat(current, {
    id: "chat",
    title: "Chat",
    createdAt: 1,
    lastMessageAt: 1,
  });
  assert.notEqual(cleared, current);
  assert.equal(cleared[0].selectedSkills, undefined);

  // The chip renders `name || command`, so a rename under a stable command has
  // to reach the list or the old label stays on screen.
  const renamed = workspaceDataProjector.upsertChat(current, {
    ...current[0],
    selectedSkills: [{ name: "Browser", command: "browser", provider: "claude", source: "builtin" }],
  });
  assert.notEqual(renamed, current);
  assert.equal(renamed[0].selectedSkills?.[0].name, "Browser");

  const swapped = workspaceDataProjector.upsertChat(current, {
    ...current[0],
    selectedSkills: [{ name: "run", command: "run", provider: "claude", source: "builtin" }],
  });
  assert.notEqual(swapped, current);
  assert.equal(swapped[0].selectedSkills?.[0].command, "run");
});
