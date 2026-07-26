import type { ScheduledTask } from "../../../models/schedule";

export function sortScheduledTasks(tasks: ScheduledTask[]): ScheduledTask[] {
  return tasks.slice().sort((left, right) => {
    if (left.enabled !== right.enabled) return left.enabled ? -1 : 1;

    const leftNext = positiveTimestamp(left.nextRunAt);
    const rightNext = positiveTimestamp(right.nextRunAt);
    if (leftNext !== rightNext) return leftNext - rightNext;

    return right.updatedAt - left.updatedAt;
  });
}

export function scheduleDefinition(task: ScheduledTask): string {
  if (task.kind === "cron") {
    return `${task.cron || "Invalid cron"} · ${task.timezone || "UTC"}`;
  }
  return task.at ? `Once · ${formatTimestamp(task.at)}` : "Once · not scheduled";
}

export function scheduleRunCount(task: ScheduledTask): string {
  return task.maxRuns
    ? `${task.runCount} of ${task.maxRuns} runs`
    : `${task.runCount} run${task.runCount === 1 ? "" : "s"}`;
}

export function canResumeScheduledTask(task: ScheduledTask): boolean {
  return !task.enabled && task.status.toLowerCase() === "paused";
}

// An agent-created task that has never fired sits parked until the user arms
// it — the enforced half of the agent-create handshake.
export function isAwaitingArm(task: ScheduledTask): boolean {
  return !!task.createdByAgent && !task.enabled && task.runCount === 0 &&
    task.status.toLowerCase() === "paused";
}

// The enable/disable button's label: armed tasks pause, parked agent-created
// tasks arm, everything else resumes.
export function toggleActionLabel(task: ScheduledTask): string {
  if (task.enabled) return "Pause";
  if (isAwaitingArm(task)) return "Arm";
  return "Resume";
}

export function formatTimestamp(timestamp?: number): string {
  if (!timestamp) return "Never";
  return new Date(timestamp).toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

function positiveTimestamp(timestamp?: number): number {
  return timestamp && timestamp > 0 ? timestamp : Number.POSITIVE_INFINITY;
}
