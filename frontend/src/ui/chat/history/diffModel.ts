export type DiffLineKind = "add" | "del" | "context" | "meta";

export interface DiffLine {
  kind: DiffLineKind;
  text: string;
  oldNo?: number;
  newNo?: number;
}

export interface DiffHunk {
  header: string;
  lines: DiffLine[];
}

export interface DiffFile {
  oldPath: string;
  newPath: string;
  binary: boolean;
  additions: number;
  deletions: number;
  hunks: DiffHunk[];
}

// parseUnifiedDiff turns `git show`-style unified diff text into per-file
// structures with line numbers, so the UI can render a real diff view instead
// of a raw <pre>. Unknown or malformed sections degrade to meta lines rather
// than throwing.
export function parseUnifiedDiff(diff: string): DiffFile[] {
  const files: DiffFile[] = [];
  let file: DiffFile | null = null;
  let hunk: DiffHunk | null = null;
  let oldNo = 0;
  let newNo = 0;

  for (const line of diff.split("\n")) {
    if (line.startsWith("diff --git ")) {
      const { oldPath, newPath } = parseGitHeaderPaths(line);
      file = { oldPath, newPath, binary: false, additions: 0, deletions: 0, hunks: [] };
      hunk = null;
      files.push(file);
      continue;
    }

    if (!file) {
      // Commit message / header block before the first file entry — skip.
      continue;
    }

    if (line.startsWith("Binary files ") || line.startsWith("GIT binary patch")) {
      file.binary = true;
      continue;
    }
    if (line.startsWith("--- ")) {
      const path = stripDiffPathPrefix(line.slice(4));
      if (path) file.oldPath = path;
      continue;
    }
    if (line.startsWith("+++ ")) {
      const path = stripDiffPathPrefix(line.slice(4));
      if (path) file.newPath = path;
      continue;
    }

    const hunkHeader = line.match(/^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
    if (hunkHeader) {
      oldNo = Number(hunkHeader[1]);
      newNo = Number(hunkHeader[2]);
      hunk = { header: line, lines: [] };
      file.hunks.push(hunk);
      continue;
    }

    if (!hunk) {
      // File-level metadata (mode changes, index lines, renames).
      continue;
    }

    if (line.startsWith("+")) {
      hunk.lines.push({ kind: "add", text: line.slice(1), newNo });
      newNo++;
      file.additions++;
    } else if (line.startsWith("-")) {
      hunk.lines.push({ kind: "del", text: line.slice(1), oldNo });
      oldNo++;
      file.deletions++;
    } else if (line.startsWith("\\")) {
      hunk.lines.push({ kind: "meta", text: line });
    } else {
      hunk.lines.push({ kind: "context", text: line.slice(1), oldNo, newNo });
      oldNo++;
      newNo++;
    }
  }

  return files;
}

// displayPath picks the human-facing path for a file entry, marking renames.
export function displayPath(file: DiffFile): string {
  if (file.oldPath && file.newPath && file.oldPath !== file.newPath) {
    if (file.oldPath === "/dev/null") return file.newPath;
    if (file.newPath === "/dev/null") return file.oldPath;
    return `${file.oldPath} → ${file.newPath}`;
  }
  return file.newPath || file.oldPath;
}

export function isNewFile(file: DiffFile): boolean {
  return file.oldPath === "/dev/null";
}

export function isDeletedFile(file: DiffFile): boolean {
  return file.newPath === "/dev/null";
}

function parseGitHeaderPaths(line: string): { oldPath: string; newPath: string } {
  // `diff --git a/path b/path`. Paths with spaces are ambiguous in this header;
  // the ---/+++ lines that follow correct them when present.
  const rest = line.slice("diff --git ".length);
  const midpoint = rest.indexOf(" b/");
  if (midpoint < 0) return { oldPath: rest, newPath: rest };
  return {
    oldPath: rest.slice(0, midpoint).replace(/^a\//, ""),
    newPath: rest.slice(midpoint + 1).replace(/^b\//, ""),
  };
}

function stripDiffPathPrefix(raw: string): string {
  const path = raw.trim().split("\t")[0];
  if (path === "/dev/null") return path;
  return path.replace(/^[ab]\//, "");
}
