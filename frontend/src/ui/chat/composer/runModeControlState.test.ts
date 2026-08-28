import assert from "node:assert/strict";
import test from "node:test";
import { isUnsupportedRunMode } from "./runModeControlState.ts";

const defaultOnly = [{ value: "default", label: "Default" }];

test("requires recovery when a saved non-default mode is unavailable", () => {
  assert.equal(isUnsupportedRunMode("plan", defaultOnly), true);
});

test("does not require recovery for default or an available mode", () => {
  assert.equal(isUnsupportedRunMode("default", defaultOnly), false);
  assert.equal(
    isUnsupportedRunMode("plan", [...defaultOnly, { value: "plan", label: "Plan" }]),
    false,
  );
});
