import { CalendarClock, Clock, Code, Folder, Monitor, Terminal } from "../../primitives/icons";
import { buildIdeUrl, defaultWorkspacePath } from "../ideLinks";
import { WorkspaceAction } from "./WorkspaceAction";

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

  return (
    <div class="flex min-w-max items-center gap-1.5">
      <div class="workspace-action-group inline-flex items-center gap-0.5 rounded-lg border border-white/10 bg-white/[0.035] p-0.5">
        <WorkspaceAction
          href={ideUrl}
          Icon={Code}
          label="IDE"
          title={`Open workspace in IDE: ${workspacePath}`}
          ariaLabel="Open workspace in IDE"
        />
        <WorkspaceAction
          onClick={onOpenTerminal}
          Icon={Terminal}
          label="Terminal"
          title={`Open terminal in container workspace: ${workspacePath}`}
          ariaLabel="Open terminal"
        />
      </div>

      <div class="workspace-action-group inline-flex items-center gap-0.5 rounded-lg border border-white/10 bg-white/[0.035] p-0.5">
        {showHistory && (
          <WorkspaceAction
            onClick={onOpenHistory}
            Icon={Clock}
            label="History"
            title="Review git history"
            ariaLabel="Review history"
          />
        )}
        <WorkspaceAction
          onClick={onOpenFiles}
          Icon={Folder}
          label="Files"
          title="Browse uploads and media files"
          ariaLabel="Open file manager"
        />
        {showSchedules && (
          <WorkspaceAction
            onClick={onOpenSchedules}
            Icon={CalendarClock}
            label="Schedules"
            title="View scheduled tasks"
            ariaLabel="Open scheduled tasks"
          />
        )}
        <WorkspaceAction
          onClick={onOpenBrowser}
          Icon={Monitor}
          label="Browser"
          title="Open browser preview"
          ariaLabel="Open browser"
          emphasized
        />
      </div>
    </div>
  );
}
