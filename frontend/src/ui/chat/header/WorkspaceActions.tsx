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
  const actionClass = `workspace-action inline-flex h-8 w-8 flex-none items-center justify-center rounded-md
                       text-ink-300 transition hover:bg-white/[0.08] hover:text-ink-100`;

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
          <WorkspaceActionIcon Icon={Code} />
        </a>
        <button
          type="button"
          onClick={onOpenTerminal}
          class={actionClass}
          title={`Open terminal in container workspace: ${workspacePath}`}
          aria-label="Open terminal"
        >
          <WorkspaceActionIcon Icon={Terminal} />
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
            <WorkspaceActionIcon Icon={Clock} />
          </button>
        )}
        <button
          type="button"
          onClick={onOpenFiles}
          class={actionClass}
          title="Browse uploads and media files"
          aria-label="Open file manager"
        >
          <WorkspaceActionIcon Icon={Folder} />
        </button>
        {showSchedules && (
          <button
            type="button"
            onClick={onOpenSchedules}
            class={actionClass}
            title="View scheduled tasks"
            aria-label="Open scheduled tasks"
          >
            <WorkspaceActionIcon Icon={CalendarClock} />
          </button>
        )}
        <button
          type="button"
          onClick={onOpenBrowser}
          class={`${actionClass} bg-accent-blue/[0.08] text-ink-100`}
          title="Open browser preview"
          aria-label="Open browser"
        >
          <WorkspaceActionIcon Icon={Monitor} />
        </button>
      </div>
    </div>
  );
}

function WorkspaceActionIcon({ Icon }: { Icon: typeof Code }) {
  return <Icon class="h-3.5 w-3.5 flex-none text-accent-blue" />;
}
