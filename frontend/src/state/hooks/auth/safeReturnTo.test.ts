import assert from "node:assert/strict";
import test from "node:test";
import { safeReturnTo } from "./safeReturnTo.ts";

const configuredOrigin = "https://remote.example.com";

test("accepts the configured origin and its project subdomains", () => {
  const originTarget = "https://remote.example.com/settings";
  const projectTarget = "https://project--3000.dev.remote.example.com/chat";

  assert.equal(safeReturnTo(originTarget, configuredOrigin), originTarget);
  assert.equal(safeReturnTo(projectTarget, configuredOrigin), projectTarget);
});

test("rejects external and deceptive return URL origins", () => {
  assert.equal(safeReturnTo("https://attacker.example/phish", configuredOrigin), "");
  assert.equal(safeReturnTo("https://remote.example.com.attacker.example/phish", configuredOrigin), "");
});
