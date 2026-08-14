import assert from "node:assert/strict";
import test from "node:test";
import type { AgentCapabilitiesCatalog } from "../../models/agentCapabilities.ts";
import { agentCapabilityState } from "./agentCapabilityState.ts";

const catalog: AgentCapabilitiesCatalog = {
  providers: [{
    provider: "codex",
    label: "Codex",
    source: "live",
    defaultMode: "default",
    modes: [{ value: "default", label: "Default" }, { value: "plan", label: "Plan" }],
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
  assert.deepEqual(state.modeOptions.map((option) => option.value), ["default", "plan"]);
});

test("falls back to the auto model for an unknown saved model", () => {
  const state = agentCapabilityState.resolve(catalog, "codex", "retired-model", false);
  assert.deepEqual(state.reasoningEffortOptions.map((option) => option.value), ["", "medium"]);
});

test("corrects selections unsupported by a live catalog", () => {
  const state = agentCapabilityState.resolve(catalog, "codex", "gpt-fast", false);
  assert.deepEqual(
    agentCapabilityState.corrections(state, {
      mode: "retired-mode",
      reasoningEffort: "ultra",
      serviceTier: "slow",
    }),
    { mode: "default", reasoningEffort: "", serviceTier: "" },
  );
});

test("preserves selections when discovery used a fallback catalog", () => {
  const fallbackCatalog: AgentCapabilitiesCatalog = {
    providers: [{ ...catalog.providers[0], source: "fallback" }],
  };
  const state = agentCapabilityState.resolve(fallbackCatalog, "codex", "gpt-fast", false);
  assert.deepEqual(
    agentCapabilityState.corrections(state, {
      mode: "retired-mode",
      reasoningEffort: "ultra",
      serviceTier: "slow",
    }),
    {},
  );
});
