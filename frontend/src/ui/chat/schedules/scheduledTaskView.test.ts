import assert from "node:assert/strict";
import test from "node:test";
import type { ScheduledTask } from "../../../models/schedule.ts";
import {
  canResumeScheduledTask,
  isAwaitingArm,
  scheduleDefinition,
  scheduleRunCount,
  sortScheduledTasks,
  toggleActionLabel,
} from "./scheduledTaskView.ts";

function task(overrides: Partial<ScheduledTask>): ScheduledTask {
  return {
    id: "task",
    name: "Task",
    ownerEmail: "owner@example.com",
    projectId: "project",
    chatId: "chat",
    prompt: "Continue.",
    kind: "cron",
    cron: "0 9 * * *",
    timezone: "America/Toronto",
    enabled: true,
    status: "scheduled",
    runCount: 0,
    createdAt: 1,
    updatedAt: 1,
    ...overrides,
  };
}

test("sortScheduledTasks prioritizes enabled tasks and their next run", () => {
  const result = sortScheduledTasks([
    task({ id: "disabled", enabled: false, nextRunAt: 1 }),
    task({ id: "later", nextRunAt: 300 }),
    task({ id: "soon", nextRunAt: 200 }),
  ]);

  assert.deepEqual(result.map((item) => item.id), ["soon", "later", "disabled"]);
});

test("scheduleDefinition describes cron schedules with their timezone", () => {
  assert.equal(
    scheduleDefinition(task({ cron: "*/10 * * * *", timezone: "UTC" })),
    "*/10 * * * * · UTC"
  );
});

test("scheduleRunCount includes a bounded task's progress", () => {
  assert.equal(scheduleRunCount(task({ runCount: 3, maxRuns: 12 })), "3 of 12 runs");
});

test("canResumeScheduledTask only permits paused tasks", () => {
  assert.equal(canResumeScheduledTask(task({ enabled: false, status: "paused" })), true);
  assert.equal(canResumeScheduledTask(task({ enabled: false, status: "completed" })), false);
  assert.equal(canResumeScheduledTask(task({ enabled: false, status: "exhausted" })), false);
  assert.equal(canResumeScheduledTask(task({ enabled: false, status: "error" })), false);
  assert.equal(canResumeScheduledTask(task({ enabled: true, status: "paused" })), false);
});

test("isAwaitingArm flags parked agent-created tasks only", () => {
  const parked = task({ createdByAgent: true, enabled: false, status: "paused", runCount: 0 });
  assert.equal(isAwaitingArm(parked), true);
  assert.equal(isAwaitingArm(task({ createdByAgent: true, enabled: true, status: "active" })), false);
  assert.equal(
    isAwaitingArm(task({ createdByAgent: true, enabled: false, status: "paused", runCount: 3 })),
    false,
  );
  assert.equal(isAwaitingArm(task({ enabled: false, status: "paused", runCount: 0 })), false);
});

test("toggleActionLabel arms parked tasks and pauses running ones", () => {
  assert.equal(toggleActionLabel(task({ enabled: true, status: "active" })), "Pause");
  assert.equal(
    toggleActionLabel(task({ createdByAgent: true, enabled: false, status: "paused", runCount: 0 })),
    "Arm",
  );
  assert.equal(toggleActionLabel(task({ enabled: false, status: "paused", runCount: 2 })), "Resume");
});
