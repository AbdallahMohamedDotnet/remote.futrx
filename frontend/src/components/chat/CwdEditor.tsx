import { Code, Database, Terminal } from "../ui/icons";

const defaultWorkspacePath = "/opt/remote.futrx.dev";
const ideBaseUrl = "https://code.remote.futrx.dev/";
const dbViewerUrl = "https://db.remote.futrx.dev/";

export function CwdEditor({
  editing,
  cwd,
  value,
  onStartEdit,
  onChange,
  onCommit,
  onCancel,
  onOpenTerminal,
}: {
  editing: boolean;
  cwd: string;
  value: string;
  onStartEdit: () => void;
  onChange: (value: string) => void;
  onCommit: () => void;
  onCancel: () => void;
  onOpenTerminal: () => void;
}) {
  const workspacePath = cwd && cwd !== "~" ? cwd : defaultWorkspacePath;
  const ideUrl = `${ideBaseUrl}?folder=${encodeURIComponent(workspacePath)}`;

  if (editing) {
    return (
      <input
        class="h-9 flex-1 min-w-[220px] bg-[#0b0d11] border border-accent-blue/70 rounded-md px-3
               text-ink-100 text-[13px] font-mono focus:outline-none"
        value={value}
        onInput={(event) => onChange((event.currentTarget as HTMLInputElement).value)}
        onBlur={onCommit}
        onKeyDown={(event) => {
          if (event.key === "Enter") onCommit();
          if (event.key === "Escape") onCancel();
        }}
        autofocus
      />
    );
  }

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
      <a
        href={dbViewerUrl}
        target="_blank"
        rel="noopener noreferrer"
        class="h-9 inline-flex items-center gap-2 px-3 rounded-md
               bg-white/5 hover:bg-white/[0.09] border border-white/10 text-left text-ink-200 flex-none"
        title="Open database viewer"
        aria-label="Open database viewer"
      >
        <Database class="w-4 h-4 text-accent-blue flex-none" />
        <span class="text-[12.5px] font-medium">Open DB</span>
      </a>
    </>
  );
}
