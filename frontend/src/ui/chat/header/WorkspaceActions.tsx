import { CalendarClock, Clock, Code, Folder, Monitor, Terminal } from "../../primitives/icons";
import { buildIdeUrl, defaultWorkspacePath } from "../ideLinks";

export function WorkspaceActions({
  cwd,
  onOpenTerminal,
  onOpenBrowser,
  onOpenHistory,
  onOpenFiles,
  onOpenSchedules,
  showHistory,
  showSchedules,
}: {
  cwd: string;
  onOpenTerminal: () => void;
  onOpenBrowser: () => void;
  onOpenHistory: () => void;
  onOpenFiles: () => void;
  onOpenSchedules: () => void;
  showHistory: boolean;
  showSchedules: boolean;
}) {
  const workspacePath = cwd && cwd !== "~" ? cwd : defaultWorkspacePath;
  const ideUrl = buildIdeUrl(workspacePath);
  const actionClass = `workspace-action h-8 inline-flex flex-none items-center gap-1.5 rounded-md px-2
                       text-left text-ink-300 transition hover:bg-white/[0.08] hover:text-ink-100`;

  return (
    <div class="flex min-w-max items-center gap-1.5">
      <div class="workspace-action-group inline-flex items-center gap-0.5 rounded-lg border border-white/10 bg-white/[0.035] p-0.5">
        <a
          href={ideUrl}
          target="_blank"
          rel="noopener noreferrer"
          class={actionClass}
          title={`Open workspace in IDE: ${workspacePath}`}
          aria-label="Open workspace in IDE"
        >
          <Code class="w-3.5 h-3.5 text-accent-blue flex-none" />
          <span class="workspace-action-label text-[11.5px] font-medium">IDE</span>
        </a>
        <button
          type="button"
          onClick={onOpenTerminal}
          class={actionClass}
          title={`Open terminal in container workspace: ${workspacePath}`}
          aria-label="Open terminal"
        >
          <Terminal class="w-3.5 h-3.5 text-accent-blue flex-none" />
          <span class="workspace-action-label text-[11.5px] font-medium">Terminal</span>
        </button>
      </div>

      <div class="workspace-action-group inline-flex items-center gap-0.5 rounded-lg border border-white/10 bg-white/[0.035] p-0.5">
        {showHistory && (
          <button
            type="button"
            onClick={onOpenHistory}
            class={actionClass}
            title="Review git history"
            aria-label="Review history"
          >
            <Clock class="w-3.5 h-3.5 text-accent-blue flex-none" />
            <span class="workspace-action-label text-[11.5px] font-medium">History</span>
          </button>
        )}
        <button
          type="button"
          onClick={onOpenFiles}
          class={actionClass}
          title="Browse uploads and media files"
          aria-label="Open file manager"
        >
          <Folder class="w-3.5 h-3.5 text-accent-blue flex-none" />
          <span class="workspace-action-label text-[11.5px] font-medium">Files</span>
        </button>
        {showSchedules && (
          <button
            type="button"
            onClick={onOpenSchedules}
            class={actionClass}
            title="View scheduled tasks"
            aria-label="Open scheduled tasks"
          >
            <CalendarClock class="w-3.5 h-3.5 text-accent-blue flex-none" />
            <span class="workspace-action-label text-[11.5px] font-medium">Schedules</span>
          </button>
        )}
        <button
          type="button"
          onClick={onOpenBrowser}
          class={`${actionClass} bg-accent-blue/[0.08] text-ink-100`}
          title="Open browser preview"
          aria-label="Open browser"
        >
          <Monitor class="w-3.5 h-3.5 text-accent-blue flex-none" />
          <span class="workspace-action-label text-[11.5px] font-medium">Browser</span>
        </button>
      </div>
    </div>
  );
}
