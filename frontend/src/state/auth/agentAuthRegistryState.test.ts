import assert from "node:assert/strict";
import test from "node:test";
import type { AgentAuthProvider } from "../../models/auth.ts";
import {
  agentAuthGateReady,
  agentAuthRevision,
  unavailableManagedAgents,
  updateAgentAuthProvider,
} from "./agentAuthRegistryState.ts";

function provider(
  id: string,
  mode: AgentAuthProvider["authentication"]["mode"],
  authenticated: boolean,
  satisfiesAccessGate = false,
): AgentAuthProvider {
  return {
    provider: id,
    label: id === "future-agent" ? "Future Agent" : id,
    executionScopes: ["host"],
    authentication: { mode, satisfiesAccessGate },
    status: { authenticated, login: { active: false } },
  };
}

test("an arbitrary managed module participates in gating and availability", () => {
  const providers = [
    provider("external-agent", "external", false),
    provider("future-agent", "managed-device", false, true),
  ];
  assert.equal(agentAuthGateReady(providers), false);
  assert.deepEqual(unavailableManagedAgents(providers, true), {
    "future-agent": "Log in to Future Agent in Settings before selecting it.",
  });

  const updated = updateAgentAuthProvider(providers, "future-agent", {
    authenticated: true,
    login: { active: false, completed: true, startedAt: 42 },
  });
  assert.equal(agentAuthGateReady(updated), true);
  assert.equal(providers[1].status.authenticated, false);
  assert.notEqual(agentAuthRevision(updated), agentAuthRevision(providers));
});

test("external modules neither block the managed gate nor receive login errors", () => {
  const providers = [provider("external-agent", "external", false)];
  assert.equal(agentAuthGateReady(providers), false);
  assert.deepEqual(unavailableManagedAgents(providers, true), {});
});

test("no-auth gate modules are immediately ready from the backend snapshot", () => {
  assert.equal(agentAuthGateReady([provider("local-agent", "none", true, true)]), true);
});
