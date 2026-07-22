import assert from "node:assert/strict";
import test from "node:test";
import type { ChatMeta } from "../../models/chat.ts";
import type { ProjectMeta } from "../../models/project.ts";
import { chatAttachmentState } from "./chatAttachmentState.ts";

const chat: ChatMeta = {
  id: "chat-1",
  title: "Chat",
  cwd: "/workspace/project/",
  createdAt: 1,
  lastMessageAt: 1,
};

const project: ProjectMeta = {
  id: "project-1",
  name: "Project",
  slug: "project",
  cwd: "/workspace",
  containerName: "project",
  status: "running",
  createdAt: 1,
  updatedAt: 1,
};

test("preserves attachment storage paths and collision-safe names", () => {
  assert.equal(chatAttachmentState.basePath(chat, []), "/workspace/project/.uploads");
  assert.equal(
    chatAttachmentState.basePath({ ...chat, projectId: project.id }, [project]),
    "/workspace/.uploads"
  );
  assert.equal(chatAttachmentState.uniqueUploadName("folder/image.png", "abc"), "image-abc.png");
  assert.equal(
    chatAttachmentState.absoluteUploadPath("/workspace/.uploads/", "folder/image-abc.png"),
    "/workspace/.uploads/image-abc.png"
  );
});
