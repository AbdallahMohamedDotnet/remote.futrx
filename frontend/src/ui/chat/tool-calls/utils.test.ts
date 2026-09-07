import assert from "node:assert/strict";
import test from "node:test";
import {
  DEFAULT_TOOL_OUTPUT_PREVIEW_CHARS,
  fullResponseErrorMessage,
  READ_TOOL_OUTPUT_PREVIEW_CHARS,
  toolOutputPreviewLimit,
} from "./utils.ts";

test("tool output expansion follows each renderer preview limit", () => {
  assert.equal(toolOutputPreviewLimit("Read"), READ_TOOL_OUTPUT_PREVIEW_CHARS);
  assert.equal(toolOutputPreviewLimit("Bash"), DEFAULT_TOOL_OUTPUT_PREVIEW_CHARS);
  assert.equal(toolOutputPreviewLimit("Grep"), DEFAULT_TOOL_OUTPUT_PREVIEW_CHARS);
  assert.equal(toolOutputPreviewLimit("future-tool"), DEFAULT_TOOL_OUTPUT_PREVIEW_CHARS);
  assert.equal(toolOutputPreviewLimit("Edit"), null);
  assert.equal(toolOutputPreviewLimit("MultiEdit"), null);
  assert.equal(toolOutputPreviewLimit("Write"), null);
});

test("full response errors always produce a useful message", () => {
  assert.equal(fullResponseErrorMessage(new Error("network failed")), "network failed");
  assert.equal(fullResponseErrorMessage("request rejected"), "request rejected");
  assert.equal(fullResponseErrorMessage({ status: 500 }), "Failed to load the full response.");
  assert.equal(fullResponseErrorMessage(null), "Failed to load the full response.");
});
