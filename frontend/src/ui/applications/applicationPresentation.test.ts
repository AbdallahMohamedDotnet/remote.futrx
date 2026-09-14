import { strict as assert } from "node:assert";
import { describe, it } from "node:test";

import type { AppApplication, AppInstance, AppKind } from "../../models/application.ts";
import {
  hasContainer,
  hasPortBinding,
  instanceSummary,
  uninstallConsequence,
} from "./applicationPresentation.ts";

// The server derives needsContainer/needsPort from the kind and ships them with
// every catalog entry, so a fixture application is one of those payloads rather than
// a kind the SPA re-interprets.
const KIND_FLAGS: Record<
  AppKind,
  { needsContainer: boolean; needsPort: boolean }
> = {
  service: { needsContainer: true, needsPort: true },
  tool: { needsContainer: true, needsPort: false },
  ui: { needsContainer: false, needsPort: false },
  backend: { needsContainer: false, needsPort: false },
};

function application(type: AppKind): AppApplication {
  return {
    id: "x",
    name: "X",
    type,
    scopes: ["project"],
    ...KIND_FLAGS[type],
  } as AppApplication;
}

function instance(overrides: Partial<AppInstance> = {}): AppInstance {
  return {
    id: "i1",
    applicationId: "x",
    name: "X",
    scope: "project",
    status: "running",
    bindAddress: "127.0.0.1",
    externalPort: 5433,
    internalPort: 5432,
    ...overrides,
  } as AppInstance;
}

describe("application presentation", () => {
  it("treats a tool as living in a container but not binding a port", () => {
    // The two questions are different for exactly one kind, which is the whole
    // reason the second predicate exists.
    assert.equal(hasContainer(application("tool")), true);
    assert.equal(hasPortBinding(application("tool")), false);
  });

  it("keeps service applications on the port presentation", () => {
    assert.equal(hasContainer(application("service")), true);
    assert.equal(hasPortBinding(application("service")), true);
  });

  it("keeps extension applications off both", () => {
    for (const kind of ["ui", "backend"] as const) {
      assert.equal(hasContainer(application(kind)), false);
      assert.equal(hasPortBinding(application(kind)), false);
    }
  });

  it("falls back to the service presentation while the catalog is loading", () => {
    assert.equal(hasContainer(undefined), true);
    assert.equal(hasPortBinding(undefined), true);
  });

  it("summarises a tool by where it runs, not by a UI it does not have", () => {
    assert.match(instanceSummary(application("tool"), true), /Workspace tool/);
    assert.match(instanceSummary(application("tool"), false), /Start it/);
    assert.match(instanceSummary(application("backend"), true), /Go plugin/);
    assert.match(instanceSummary(application("ui"), true), /Interface extension/);
  });

  it("never promises to release a host port a tool never held", () => {
    const message = uninstallConsequence(instance(), application("tool"));
    assert.doesNotMatch(message, /port/i);
    assert.match(message, /project container/);
  });

  it("still reports the released port for a service", () => {
    const message = uninstallConsequence(instance(), application("service"));
    assert.match(message, /127\.0\.0\.1:5433/);
  });

  it("says nothing is removed from a container for an extension", () => {
    const message = uninstallConsequence(instance(), application("ui"));
    assert.match(message, /Nothing is removed from any container/);
  });

});
