import assert from "node:assert/strict";
import test from "node:test";
import type { ChatMeta } from "../models/chat.ts";
import type { ProjectMeta } from "../models/project.ts";
import { STORAGE_KEYS } from "../config/storageKeys.ts";
import { workspaceSidebarService } from "./workspaceSidebarService.ts";

const projects: ProjectMeta[] = [
  {
    id: "older",
    name: "Older project",
    slug: "older-project",
    cwd: "/older",
    containerName: "older",
    status: "running",
    order: 1,
    createdAt: 1,
    updatedAt: 1,
  },
  {
    id: "newer",
    name: "Newer project",
    slug: "newer-project",
    cwd: "/newer",
    containerName: "newer",
    status: "running",
    order: 2,
    createdAt: 2,
    updatedAt: 2,
  },
];

const chats: ChatMeta[] = [
  { id: "old-chat", title: "Old", projectId: "newer", createdAt: 1, lastMessageAt: 1 },
  { id: "new-chat", title: "New", projectId: "newer", createdAt: 2, lastMessageAt: 2 },
  { id: "loose", title: "Loose", createdAt: 3, lastMessageAt: 3 },
];

test("orders projects and their chats by recency", () => {
  const model = workspaceSidebarService.model(chats, projects, "");
  assert.deepEqual(model.visibleProjects.map((node) => node.project.id), ["newer", "older"]);
  assert.deepEqual(model.visibleProjects[0].chats.map((chat) => chat.id), ["new-chat", "old-chat"]);
  assert.deepEqual(model.visibleLooseChats.map((chat) => chat.id), ["loose"]);
});

test("a deleted active chat hands over to the next chat, not the empty state", () => {
  const remaining = chats.filter((chat) => chat.id !== "new-chat");
  assert.equal(workspaceSidebarService.isActiveChatMissing(remaining, "new-chat"), true);
  assert.equal(workspaceSidebarService.replacementChatId(remaining), "old-chat");
  // Same pick a fresh load would make, so the handover is not a special case.
  assert.equal(workspaceSidebarService.initialChatId(true, null, remaining), "old-chat");
  // Deleting the last chat is the one case that legitimately clears selection.
  assert.equal(workspaceSidebarService.replacementChatId([]), null);
});

test("project drag-reorder respects which side of the target it was dropped on", () => {
  const ids = ["a", "b", "c"];
  const reorder = workspaceSidebarService.reorderProjectIds.bind(workspaceSidebarService);

  // Dropping past the last project must land last. Splicing at the target's
  // pre-removal index used to leave it one slot short, at ["b", "a", "c"].
  assert.deepEqual(reorder(ids, "a", "c", "after"), ["b", "c", "a"]);
  assert.deepEqual(reorder(ids, "a", "c", "before"), ["b", "a", "c"]);
  assert.deepEqual(reorder(ids, "c", "a", "before"), ["c", "a", "b"]);
  assert.deepEqual(reorder(ids, "c", "a", "after"), ["a", "c", "b"]);

  // Drops that change nothing report null so no reorder request is sent.
  assert.equal(reorder(ids, "a", "a", "before"), null);
  assert.equal(reorder(ids, "a", "b", "before"), null);
  assert.equal(reorder(ids, "b", "a", "after"), null);
  assert.equal(reorder(ids, "a", "missing", "after"), null);
});

test("an expanded project stays expanded after a reload", () => {
  const store = new Map<string, string>();
  (globalThis as { localStorage?: unknown }).localStorage = {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, value),
  };

  // "newer" has unread chats, "older" does not: the first-sight seeding folds
  // "older" and leaves "newer" open.
  const seeded = workspaceSidebarService.collapsedProjects(projects, chats, {});
  assert.deepEqual(seeded, { older: true, newer: false });

  // The user expands "older" and folds "newer"; both choices are written out.
  const chosen = { ...seeded, older: false, newer: true };
  workspaceSidebarService.writeCollapsedProjects(chosen);

  // A reload starts from what was stored, and seeding leaves those entries be.
  const restored = workspaceSidebarService.readCollapsedProjects();
  assert.deepEqual(restored, chosen);
  assert.deepEqual(
    workspaceSidebarService.collapsedProjects(projects, chats, restored),
    chosen
  );

  // Junk in storage falls back to seeding rather than breaking the sidebar.
  store.set(STORAGE_KEYS.collapsedProjects, "not json");
  assert.deepEqual(workspaceSidebarService.readCollapsedProjects(), {});
  store.set(STORAGE_KEYS.collapsedProjects, '{"older":"yes"}');
  assert.deepEqual(workspaceSidebarService.readCollapsedProjects(), {});
});
