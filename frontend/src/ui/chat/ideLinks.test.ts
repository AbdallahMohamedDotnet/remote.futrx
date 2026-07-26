import assert from "node:assert/strict";
import test from "node:test";
import { openFilePayload } from "./ideLinks.ts";

test("openFilePayload targets a file at line and column", () => {
  const payload = openFilePayload("code.remote.futrx.dev", "/workspace/docs/flow.md", 92, 5);
  assert.deepEqual(JSON.parse(payload), [
    ["openFile", "vscode-remote://code.remote.futrx.dev/workspace/docs/flow.md:92:5"],
    ["gotoLineMode", "true"],
  ]);
});

test("openFilePayload with a line only omits the column", () => {
  const payload = openFilePayload("code.remote.futrx.dev", "/workspace/docs/flow.md", 92);
  assert.deepEqual(JSON.parse(payload), [
    ["openFile", "vscode-remote://code.remote.futrx.dev/workspace/docs/flow.md:92"],
    ["gotoLineMode", "true"],
  ]);
});

test("openFilePayload without a line skips gotoLineMode", () => {
  const payload = openFilePayload("code.remote.futrx.dev", "/workspace/README.md");
  assert.deepEqual(JSON.parse(payload), [
    ["openFile", "vscode-remote://code.remote.futrx.dev/workspace/README.md"],
  ]);
});

test("openFilePayload percent-encodes special path segments", () => {
  const payload = openFilePayload("code.remote.futrx.dev", "/workspace/a b/no#te.md", 3);
  const [openFile] = JSON.parse(payload) as [string, string][];
  assert.equal(openFile[1], "vscode-remote://code.remote.futrx.dev/workspace/a%20b/no%23te.md:3");
});
