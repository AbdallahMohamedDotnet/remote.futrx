import assert from "node:assert/strict";
import test from "node:test";
import { createChatComposerSessionStore } from "./composerSessionStore.ts";

function fakeStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial));
  return {
    data,
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => {
      data.set(key, value);
    },
  };
}

const KEY = "remote.futrx.composerSession.v1";
const PREFERENCES = { provider: "claude", mode: "default" };

test("drafts and queues persist across store instances", () => {
  const storage = fakeStorage();
  const first = createChatComposerSessionStore(storage);
  first.getState().setDraft("chat-1", "half-typed message");
  first.getState().setQueuedPrompts("chat-1", [{
    id: "q1",
    text: "queued while agent ran tools",
    preferences: PREFERENCES,
  }]);

  const second = createChatComposerSessionStore(storage);
  assert.equal(second.getState().drafts.get("chat-1"), "half-typed message");
  assert.deepEqual(second.getState().promptQueues.get("chat-1"), [
    { id: "q1", text: "queued while agent ran tools", preferences: PREFERENCES },
  ]);
});

test("clearing a queue removes it from the snapshot", () => {
  const storage = fakeStorage();
  const store = createChatComposerSessionStore(storage);
  store.getState().setQueuedPrompts("chat-1", [{ id: "q1", text: "one", preferences: PREFERENCES }]);
  store.getState().setQueuedPrompts("chat-1", []);

  const reloaded = createChatComposerSessionStore(storage);
  assert.deepEqual(reloaded.getState().promptQueues.get("chat-1") ?? [], []);
});

test("corrupt snapshots are ignored", () => {
  const storage = fakeStorage({ [KEY]: "{not json" });
  const store = createChatComposerSessionStore(storage);
  assert.equal(store.getState().drafts.get("chat-1") ?? "", "");
  assert.deepEqual(store.getState().promptQueues.get("chat-1") ?? [], []);
});

test("legacy queue entries are restored to the draft instead of auto-running", () => {
  const storage = fakeStorage({
    [KEY]: JSON.stringify({
      drafts: { "chat-1": "draft" },
      queues: { "chat-1": [{ id: "q1", text: "review me" }, { id: 7 }, null, "junk"] },
    }),
  });
  const store = createChatComposerSessionStore(storage);
  assert.equal(store.getState().drafts.get("chat-1"), "review me\n\ndraft");
  assert.deepEqual(store.getState().promptQueues.get("chat-1") ?? [], []);
});

test("store works without any storage backend", () => {
  const store = createChatComposerSessionStore(null);
  store.getState().setDraft("chat-1", "memory only");
  assert.equal(store.getState().drafts.get("chat-1"), "memory only");
  store.getState().setQueuedPrompts("chat-1", [{
    id: "q1",
    text: "queued",
    preferences: PREFERENCES,
  }]);
  assert.deepEqual(store.getState().promptQueues.get("chat-1"), [
    { id: "q1", text: "queued", preferences: PREFERENCES },
  ]);
});
