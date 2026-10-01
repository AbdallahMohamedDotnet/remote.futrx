import assert from "node:assert/strict";
import test from "node:test";
import { projectPreviewUrlService } from "./projectPreviewUrlService.ts";

const publicHostname = "remote.example.com";

test("builds a preview URL beneath the runtime public hostname", () => {
  assert.equal(
    projectPreviewUrlService.build("demo", 4173, publicHostname),
    "https://dev--demo--4173.remote.example.com",
  );
});

test("extracts and validates only URLs for the runtime public hostname", () => {
  const expected = "https://dev--demo--4173.remote.example.com/path";
  const urls = projectPreviewUrlService.findInText(
    `custom ${expected}. production https://dev--demo--4173.remote.futrx.com`,
    publicHostname,
  );

  assert.deepEqual(urls, [expected]);
  assert.equal(projectPreviewUrlService.belongsToProject(expected, "demo", publicHostname), true);
  assert.equal(
    projectPreviewUrlService.belongsToProject(
      "https://dev--demo--4173.remote.futrx.com",
      "demo",
      publicHostname,
    ),
    false,
  );
});

test("rejects invalid preview ports", () => {
  assert.equal(
    projectPreviewUrlService.belongsToProject(
      "https://dev--demo--1023.remote.example.com",
      "demo",
      publicHostname,
    ),
    false,
  );
  assert.equal(projectPreviewUrlService.port("https://dev--demo--4173.remote.example.com"), 4173);
});
