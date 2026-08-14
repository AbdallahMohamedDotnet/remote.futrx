import assert from "node:assert/strict";
import test from "node:test";
import type { AgentCapabilitiesCatalog } from "../../models/agentCapabilities.ts";
import { agentCapabilityState } from "./agentCapabilityState.ts";

const catalog: AgentCapabilitiesCatalog = {
  providers: [{
    provider: "codex",
    label: "Codex",
    source: "live",
    defaultMode: "code",
    modes: [{ value: "code", label: "Code" }, { value: "plan", label: "Plan" }],
    models: [
      {
        id: "",
        label: "Auto",
        reasoningEfforts: [{ value: "", label: "Auto" }, { value: "medium", label: "Medium" }],
        serviceTiers: [{ value: "", label: "Auto" }],
      },
      {
        id: "gpt-fast",
        label: "GPT Fast",
        reasoningEfforts: [{ value: "", label: "Auto" }, { value: "low", label: "Low" }],
        serviceTiers: [{ value: "", label: "Auto" }, { value: "priority", label: "Fast" }],
      },
    ],
  }],
};

test("resolves thinking and speed from the selected model", () => {
  const state = agentCapabilityState.resolve(catalog, "codex", "gpt-fast", false);
  assert.deepEqual(state.reasoningEffortOptions.map((option) => option.value), ["", "low"]);
  assert.deepEqual(state.serviceTierOptions.map((option) => option.value), ["", "priority"]);
  assert.deepEqual(state.modeOptions.map((option) => option.value), ["code", "plan"]);
});

test("falls back to the auto model for an unknown saved model", () => {
  const state = agentCapabilityState.resolve(catalog, "codex", "retired-model", false);
  assert.deepEqual(state.reasoningEffortOptions.map((option) => option.value), ["", "medium"]);
});
