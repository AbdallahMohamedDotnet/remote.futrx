import { useId, useState } from "preact/hooks";
import { CalendarClock, Clock, Code, Folder, Monitor, Terminal } from "../../primitives/icons";
import { buildIdeUrl, defaultWorkspacePath } from "../ideLinks";

const actionClass = `workspace-action relative inline-flex h-9 w-9 flex-none items-center justify-center rounded-md
                     border border-white/10 bg-white/5 text-ink-200 transition hover:bg-white/[0.09] hover:text-ink-100
                     focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-blue/80`;

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
    <div class="flex flex-col items-center gap-2">
      <WorkspaceAction
        Icon={Code}
        href={ideUrl}
        label="Workspace IDE"
        tooltip="Open workspace in IDE"
      />
      <WorkspaceAction
        Icon={Terminal}
        onClick={onOpenTerminal}
        label="Container terminal"
        tooltip="Open container terminal"
      />
      {showHistory && (
        <WorkspaceAction
          Icon={Clock}
          onClick={onOpenHistory}
          label="Git history"
          tooltip="Review git history"
        />
      )}
      <WorkspaceAction
        Icon={Folder}
        onClick={onOpenFiles}
        label="Workspace files"
        tooltip="Browse workspace files"
      />
      {showSchedules && (
        <WorkspaceAction
          Icon={CalendarClock}
          onClick={onOpenSchedules}
          label="Scheduled tasks"
          tooltip="View scheduled tasks"
        />
      )}
      <WorkspaceAction
        Icon={Monitor}
        onClick={onOpenBrowser}
        label="Browser preview"
        tooltip="Open browser preview"
      />
    </div>
  );
}

function WorkspaceAction({
  Icon,
  label,
  tooltip,
  href,
  onClick,
}: {
  Icon: typeof Code;
  label: string;
  tooltip: string;
  href?: string;
  onClick?: () => void;
}) {
  const tooltipId = useId();
  const [isHovered, setIsHovered] = useState(false);
  const [isFocused, setIsFocused] = useState(false);
  const [isDismissed, setIsDismissed] = useState(false);
  const isTooltipOpen = !isDismissed && (isHovered || isFocused);
  const interactionProps = {
    "aria-describedby": tooltipId,
    "aria-label": label,
    onBlur: () => {
      setIsFocused(false);
      setIsDismissed(false);
    },
    onFocus: () => {
      setIsFocused(true);
      setIsDismissed(false);
    },
    onKeyDown: (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setIsDismissed(true);
      event.stopPropagation();
    },
    onMouseEnter: () => {
      setIsHovered(true);
      setIsDismissed(false);
    },
    onMouseLeave: () => setIsHovered(false),
  };
  const content = (
    <>
      <Icon aria-hidden="true" focusable="false" class="h-4 w-4 flex-none" />
      <span
        id={tooltipId}
        role="tooltip"
        class={`workspace-action-tooltip pointer-events-none absolute right-full top-1/2 z-50 mr-2 -translate-y-1/2 whitespace-nowrap rounded-md border border-white/10 bg-[#191a1f] px-2 py-1.5 text-[11px] font-medium text-ink-100 shadow-xl transition-[opacity,transform] duration-150 motion-reduce:transition-none ${
          isTooltipOpen ? "translate-x-0 opacity-100" : "translate-x-1 opacity-0"
        }`}
      >
        {tooltip}
      </span>
    </>
  );

  if (href) {
    return (
      <a
        {...interactionProps}
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        class={actionClass}
      >
        {content}
      </a>
    );
  }

  return (
    <button {...interactionProps} type="button" onClick={onClick} class={actionClass}>
      {content}
    </button>
  );
}
