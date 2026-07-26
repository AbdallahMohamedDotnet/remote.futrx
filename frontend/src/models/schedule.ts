export type ScheduledTaskKind = "once" | "cron";

export interface ScheduledTask {
  id: string;
  name: string;
  ownerEmail: string;
  projectId: string;
  chatId: string;
  prompt: string;
  kind: ScheduledTaskKind;
  at?: number;
  cron?: string;
  timezone: string;
  enabled: boolean;
  status: string;
  nextRunAt?: number;
  runCount: number;
  maxRuns?: number;
  lastRunAt?: number;
  lastRunStatus?: string;
  lastError?: string;
  createdByAgent?: boolean;
  createdAt: number;
  updatedAt: number;
}

export interface CreateScheduledTaskInput {
  name: string;
  prompt: string;
  kind: ScheduledTaskKind;
  at?: number;
  cron?: string;
  timezone: string;
  maxRuns?: number;
}

export interface UpdateScheduledTaskInput {
  enabled?: boolean;
  name?: string;
  prompt?: string;
  at?: number;
  cron?: string;
  timezone?: string;
  maxRuns?: number;
}
