import assert from "node:assert/strict";
import test from "node:test";
import { fileOpenAction, viewableMediaKind } from "./fileMeta.ts";

test("viewableMediaKind mirrors the backend inline media set", () => {
  assert.equal(viewableMediaKind("shot.PNG"), "image");
  assert.equal(viewableMediaKind("demo.mp4"), "video");
  assert.equal(viewableMediaKind("voice.m4a"), "audio");
  assert.equal(viewableMediaKind("report.pdf"), "pdf");
  assert.equal(viewableMediaKind("app.tsx"), null);
  assert.equal(viewableMediaKind("archive.zip"), null);
  assert.equal(viewableMediaKind("clip.mkv"), null);
  assert.equal(viewableMediaKind("noextension"), null);
});

test("fileOpenAction opens media in the viewer", () => {
  assert.deepEqual(fileOpenAction("shot.png"), { action: "media", kind: "image" });
  assert.deepEqual(fileOpenAction("report.pdf"), { action: "media", kind: "pdf" });
});

test("fileOpenAction opens source and text files in the IDE", () => {
  assert.deepEqual(fileOpenAction("main.go"), { action: "ide" });
  assert.deepEqual(fileOpenAction("data.json"), { action: "ide" });
  assert.deepEqual(fileOpenAction("README.md"), { action: "ide" });
  assert.deepEqual(fileOpenAction("Makefile"), { action: "ide" });
});

test("fileOpenAction downloads archives and unrenderable media", () => {
  assert.deepEqual(fileOpenAction("bundle.zip"), { action: "download" });
  assert.deepEqual(fileOpenAction("clip.mkv"), { action: "download" });
  assert.deepEqual(fileOpenAction("photo.heic"), { action: "download" });
});
