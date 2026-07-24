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
