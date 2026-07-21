import { Clock, Code, Folder, Monitor, Terminal } from "../../primitives/icons";
import { buildIdeUrl, defaultWorkspacePath } from "../ideLinks";

export function WorkspaceActions({
  cwd,
  onOpenTerminal,
  onOpenBrowser,
  onOpenHistory,
  onOpenFiles,
}: {
  cwd: string;
  onOpenTerminal: () => void;
  onOpenBrowser: () => void;
  onOpenHistory: () => void;
  onOpenFiles: () => void;
}) {
  const workspacePath = cwd && cwd !== "~" ? cwd : defaultWorkspacePath;
  const ideUrl = buildIdeUrl(workspacePath);

  return (
    <>
      <a
        href={ideUrl}
        target="_blank"
        rel="noopener noreferrer"
        class="h-9 max-w-[72vw] md:max-w-[520px] inline-flex items-center gap-2 px-3 rounded-md
               bg-white/5 hover:bg-white/[0.09] border border-white/10 text-left text-ink-200"
        title={`Open workspace in IDE: ${workspacePath}`}
        aria-label="Open workspace in IDE"
      >
        <Code class="w-4 h-4 text-accent-blue flex-none" />
        <span class="text-[12.5px] font-medium">Open in IDE</span>
      </a>
      <button
        type="button"
        onClick={onOpenTerminal}
        class="h-9 inline-flex items-center gap-2 px-3 rounded-md
               bg-white/5 hover:bg-white/[0.09] border border-white/10 text-left text-ink-200 flex-none"
        title={`Open terminal in container workspace: ${workspacePath}`}
        aria-label="Open terminal"
      >
        <Terminal class="w-4 h-4 text-accent-blue flex-none" />
        <span class="text-[12.5px] font-medium">Open Terminal</span>
      </button>
      <button
        type="button"
        onClick={onOpenHistory}
        class="h-9 inline-flex items-center gap-2 px-3 rounded-md
               bg-white/5 hover:bg-white/[0.09] border border-white/10 text-left text-ink-200 flex-none ml-auto"
        title="Review git history"
        aria-label="Review history"
      >
        <Clock class="w-4 h-4 text-accent-blue flex-none" />
        <span class="text-[12.5px] font-medium">History</span>
      </button>
      <button
        type="button"
        onClick={onOpenFiles}
        class="h-9 inline-flex items-center gap-2 px-3 rounded-md
               bg-white/5 hover:bg-white/[0.09] border border-white/10 text-left text-ink-200 flex-none"
        title="Browse uploads and media files"
        aria-label="Open file manager"
      >
        <Folder class="w-4 h-4 text-accent-blue flex-none" />
        <span class="text-[12.5px] font-medium">Files</span>
      </button>
      <button
        type="button"
        onClick={onOpenBrowser}
        class="h-9 inline-flex items-center gap-2 px-3 rounded-md
               bg-white/5 hover:bg-white/[0.09] border border-white/10 text-left text-ink-200 flex-none"
        title="Open browser preview"
        aria-label="Open browser"
      >
        <Monitor class="w-4 h-4 text-accent-blue flex-none" />
        <span class="text-[12.5px] font-medium">Open Browser</span>
      </button>
    </>
  );
}
