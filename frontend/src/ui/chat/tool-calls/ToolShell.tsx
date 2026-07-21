import type { ComponentChildren } from "preact";
import { useState } from "preact/hooks";
import { AlertCircle, ChevronDown, ChevronRight, Loader } from "../../primitives/icons";

export function ToolShell({
  icon,
  label,
  badge,
  status,
  isError,
  defaultOpen,
  children,
}: {
  icon: ComponentChildren;
  label: ComponentChildren;
  badge?: string;
  status: "running" | "done";
  isError?: boolean;
  defaultOpen?: boolean;
  children?: ComponentChildren;
}) {
  const [open, setOpen] = useState(!!defaultOpen);
  return (
    <div class={`codex-tool-shell my-2 border rounded-lg overflow-hidden text-sm shadow-sm
                ${isError ? "border-accent-red/50 bg-accent-red/5" : "border-white/10 bg-[#101318]"}`}>
      <button
        type="button"
        onClick={() => setOpen((current) => !current)}
        class={`codex-tool-trigger w-full min-h-10 flex items-center gap-2 px-3 py-2 text-left
                ${isError ? "bg-accent-red/10" : "bg-white/[0.03] hover:bg-white/[0.06]"}`}
      >
        {children ? (
          open ? <ChevronDown class="w-3.5 h-3.5 text-ink-300 flex-none" /> : <ChevronRight class="w-3.5 h-3.5 text-ink-300 flex-none" />
        ) : (
          <span class="w-3.5 flex-none" />
        )}
        <span class={`flex-none ${isError ? "text-accent-red" : "text-accent-blue"}`}>{icon}</span>
        <span class="flex-1 truncate text-ink-100">{label}</span>
        {badge && <span class="text-[11px] text-ink-300 flex-none">{badge}</span>}
        {status === "running" ? (
          <Loader class="w-3.5 h-3.5 text-ink-300 animate-spin flex-none" />
        ) : isError ? (
          <AlertCircle class="w-3.5 h-3.5 text-accent-red flex-none" />
        ) : null}
      </button>
      {open && children && (
        <div class="border-t border-white/10 bg-[#0b0d11]">
          {children}
        </div>
      )}
    </div>
  );
}
