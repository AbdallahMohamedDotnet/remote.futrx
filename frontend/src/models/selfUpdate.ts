export interface SelfUpdateCheck {
  checkedAt: number;
  latestTag?: string;
  updateAvailable: boolean;
  error?: string;
}

export interface SelfUpdateRun {
  state: "running" | "succeeded" | "failed";
  target: string;
  startedAt: number;
  startedBy?: string;
  finishedAt?: number;
  exitCode?: number;
  log?: string;
}

export interface SelfUpdateStatus {
  currentVersion: string;
  lastCheck?: SelfUpdateCheck | null;
  run?: SelfUpdateRun | null;
}
