export interface GitHistoryRepo {
  id: string;
  name: string;
  path: string;
  relativePath: string;
  currentRef: string;
  currentSha: string;
  dirty: boolean;
  dirtyFiles?: string[];
}

export interface GitHistoryCommit {
  sha: string;
  shortSha: string;
  subject: string;
  authorName: string;
  authorEmail: string;
  authorDate: number;
  isHead: boolean;
}

export interface GitHistoryReposResponse {
  workspaceRoot: string;
  repos: GitHistoryRepo[];
}

export interface GitHistoryCommitsResponse {
  repo: GitHistoryRepo;
  commits: GitHistoryCommit[];
}

export interface GitHistoryDiffResponse {
  repo: GitHistoryRepo;
  commit: GitHistoryCommit;
  diff: string;
  truncated: boolean;
}

export interface GitHistoryCheckoutResponse {
  repo: GitHistoryRepo;
  output?: string;
  checkpointSha?: string;
}

export class DirtyWorkingTreeError extends Error {
  dirtyFiles: string[];

  constructor(message: string, dirtyFiles: string[] = []) {
    super(message);
    this.name = "DirtyWorkingTreeError";
    this.dirtyFiles = dirtyFiles;
  }
}
