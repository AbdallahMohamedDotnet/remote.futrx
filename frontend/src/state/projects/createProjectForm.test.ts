import assert from "node:assert/strict";
import test from "node:test";
import type { ProjectMeta } from "../../models/project.ts";
import { createProjectForm } from "./createProjectForm.ts";

function project(overrides: Partial<ProjectMeta>): ProjectMeta {
  return {
    id: "id",
    name: "Name",
    slug: "name",
    cwd: "/var/lib/remote/projects/name/workspace",
    containerName: "name",
    status: "running",
    createdAt: 1,
    updatedAt: 1,
    ...overrides,
  };
}

test("slugify mirrors the backend rules", () => {
  assert.equal(createProjectForm.slugify("My Project"), "my-project");
  assert.equal(createProjectForm.slugify("  FutrX Web  "), "futrx-web");
  assert.equal(createProjectForm.slugify("a_b.c/d"), "a-b-c-d");
  assert.equal(createProjectForm.slugify("123 go"), "p-123-go");
  assert.equal(createProjectForm.slugify("!!!"), "");
  assert.equal(createProjectForm.slugify("trailing---"), "trailing");
  const long = createProjectForm.slugify("x".repeat(50));
  assert.equal(long.length, createProjectForm.maxSlugLen);
});

test("validate rejects empty, short, and taken names", () => {
  const projects = [project({ name: "futrx-web", slug: "futrx-web" })];
  assert.equal(createProjectForm.validate("", projects).ok, false);
  assert.equal(createProjectForm.validate("", projects).message, "");
  assert.equal(
    createProjectForm.validate("!", projects).message,
    "Use at least 2 letters or numbers."
  );
  assert.equal(
    createProjectForm.validate("FutrX Web", projects).message,
    "A project named futrx-web already exists."
  );
  const ok = createProjectForm.validate("Ops Runbook", projects);
  assert.deepEqual(ok, { ok: true, slug: "ops-runbook", message: "Saved as ops-runbook" });
  assert.equal(createProjectForm.validate("plain", projects).message, "");
});

test("pathPreview derives the workspace root from an existing project", () => {
  const projects = [
    project({ slug: "futrx-web", cwd: "/var/lib/remote/projects/futrx-web/workspace" }),
  ];
  assert.equal(
    createProjectForm.pathPreview(projects, "ops"),
    "/var/lib/remote/projects/ops/workspace"
  );
  assert.equal(
    createProjectForm.pathPreview(projects, ""),
    "/var/lib/remote/projects/…/workspace"
  );
  assert.equal(createProjectForm.pathPreview([], "ops"), "~/projects/ops");
  assert.equal(createProjectForm.pathPreview([], ""), "~/projects/…");
});
