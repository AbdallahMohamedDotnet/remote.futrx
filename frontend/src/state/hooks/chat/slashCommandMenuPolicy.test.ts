import assert from "node:assert/strict";
import test from "node:test";
import type { RegisteredSkill } from "../../../models/skill.ts";
import { slashCommandMenuPolicy } from "./slashCommandMenuPolicy.ts";

const skills: RegisteredSkill[] = [
  {
    name: "Deploy",
    command: "ship",
    description: "Release to QA",
    provider: "codex",
    source: "user",
  },
  {
    name: "Review",
    description: "Inspect architecture",
    provider: "codex",
    source: "system",
  },
];

test("opens only for a leading command token without whitespace", () => {
  assert.equal(slashCommandMenuPolicy.resolve("", skills), null);
  assert.equal(slashCommandMenuPolicy.resolve("deploy", skills), null);
  assert.equal(slashCommandMenuPolicy.resolve(" /deploy", skills), null);
  assert.equal(slashCommandMenuPolicy.resolve("/deploy now", skills), null);

  const menu = slashCommandMenuPolicy.resolve("/", skills);
  assert.equal(menu?.query, "");
  assert.equal(menu?.items, skills);
});

test("filters every searchable skill field without changing source order", () => {
  assert.deepEqual(slashCommandMenuPolicy.resolve("/DEP", skills)?.items, [skills[0]]);
  assert.deepEqual(slashCommandMenuPolicy.resolve("/ship", skills)?.items, [skills[0]]);
  assert.deepEqual(slashCommandMenuPolicy.resolve("/architecture", skills)?.items, [skills[1]]);
  assert.deepEqual(slashCommandMenuPolicy.resolve("/SYSTEM", skills)?.items, [skills[1]]);
  assert.deepEqual(slashCommandMenuPolicy.resolve("/missing", skills)?.items, []);
});

test("clamps a stale highlight to the final visible item", () => {
  assert.equal(slashCommandMenuPolicy.clampHighlight(3, 2), 1);
  assert.equal(slashCommandMenuPolicy.clampHighlight(1, 2), 1);
  assert.equal(slashCommandMenuPolicy.clampHighlight(3, 0), 0);
});

test("moves the highlight cyclically and holds at zero for an empty menu", () => {
  assert.equal(slashCommandMenuPolicy.moveHighlight(0, 1, 2), 1);
  assert.equal(slashCommandMenuPolicy.moveHighlight(1, 1, 2), 0);
  assert.equal(slashCommandMenuPolicy.moveHighlight(0, -1, 2), 1);
  assert.equal(slashCommandMenuPolicy.moveHighlight(1, -1, 2), 0);
  assert.equal(slashCommandMenuPolicy.moveHighlight(4, 1, 0), 0);
});
