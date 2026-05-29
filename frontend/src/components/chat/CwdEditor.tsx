import { Folder } from "../ui/icons";

export function CwdEditor({
  editing,
  cwd,
  value,
  onStartEdit,
  onChange,
  onCommit,
  onCancel,
}: {
  editing: boolean;
  cwd: string;
  value: string;
  onStartEdit: () => void;
  onChange: (value: string) => void;
  onCommit: () => void;
  onCancel: () => void;
}) {
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
    <button
      type="button"
      class="h-9 max-w-[72vw] md:max-w-[520px] inline-flex items-center gap-2 px-3 rounded-md
             bg-white/5 hover:bg-white/[0.09] border border-white/10 text-left text-ink-200"
      onClick={onStartEdit}
      title="Change working directory"
    >
      <Folder class="w-4 h-4 text-accent-blue flex-none" />
      <span class="truncate font-mono text-[12.5px]">{cwd}</span>
    </button>
  );
}
