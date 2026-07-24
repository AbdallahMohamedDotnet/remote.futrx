import assert from "node:assert/strict";
import test from "node:test";
import { displayPath, isDeletedFile, isNewFile, parseUnifiedDiff } from "./diffModel.ts";

const sample = [
  "commit abc123",
  "Author: Dev <dev@example.com>",
  "",
  "    fix: adjust greeting",
  "",
  "diff --git a/src/app.ts b/src/app.ts",
  "index 111..222 100644",
  "--- a/src/app.ts",
  "+++ b/src/app.ts",
  "@@ -1,3 +1,4 @@",
  " const a = 1;",
  "-console.log(\"hi\");",
  "+console.log(\"hello\");",
  "+console.log(\"world\");",
  " export {};",
  "diff --git a/assets/logo.png b/assets/logo.png",
  "Binary files a/assets/logo.png and b/assets/logo.png differ",
  "diff --git a/NEW.md b/NEW.md",
  "new file mode 100644",
  "--- /dev/null",
  "+++ b/NEW.md",
  "@@ -0,0 +1,2 @@",
  "+# New",
  "+content",
  "\\ No newline at end of file",
].join("\n");

test("parseUnifiedDiff splits files and counts changes", () => {
  const files = parseUnifiedDiff(sample);
  assert.equal(files.length, 3);

  const [app, logo, added] = files;
  assert.equal(app.newPath, "src/app.ts");
  assert.equal(app.additions, 2);
  assert.equal(app.deletions, 1);
  assert.equal(app.hunks.length, 1);
  assert.equal(logo.binary, true);
  assert.equal(added.additions, 2);
  assert.equal(isNewFile(added), true);
  assert.equal(isDeletedFile(added), false);
});

test("parseUnifiedDiff tracks line numbers through a hunk", () => {
  const [app] = parseUnifiedDiff(sample);
  const lines = app.hunks[0].lines;
  assert.deepEqual(
    lines.map((line) => [line.kind, line.oldNo ?? null, line.newNo ?? null]),
    [
      ["context", 1, 1],
      ["del", 2, null],
      ["add", null, 2],
      ["add", null, 3],
      ["context", 3, 4],
    ],
  );
});

test("parseUnifiedDiff keeps no-newline markers as meta lines", () => {
  const files = parseUnifiedDiff(sample);
  const markers = files[2].hunks[0].lines.filter((line) => line.kind === "meta");
  assert.equal(markers.length, 1);
  assert.match(markers[0].text, /No newline/);
});

test("parseUnifiedDiff handles empty input", () => {
  assert.deepEqual(parseUnifiedDiff(""), []);
});

test("displayPath marks renames and resolves added/deleted files", () => {
  const renamed = parseUnifiedDiff(
    ["diff --git a/old.ts b/new.ts", "--- a/old.ts", "+++ b/new.ts", "@@ -1 +1 @@", "-a", "+b"].join("\n"),
  )[0];
  assert.equal(displayPath(renamed), "old.ts → new.ts");

  const added = parseUnifiedDiff(
    ["diff --git a/x.md b/x.md", "--- /dev/null", "+++ b/x.md", "@@ -0,0 +1 @@", "+hi"].join("\n"),
  )[0];
  assert.equal(displayPath(added), "x.md");
});
