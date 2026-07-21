import { MessageSquare } from "../../ui/icons";

export function ThreadEmptyState({ cwd }: { cwd?: string }) {
  return (
    <div class="text-center text-ink-300 text-sm py-12 px-4 max-w-md mx-auto">
      <div class="w-14 h-14 mx-auto mb-4 rounded-lg bg-white/[0.06] border border-white/10 grid place-items-center">
        <MessageSquare class="w-7 h-7 opacity-70" />
      </div>
      <div class="font-semibold text-ink-100 text-base">Start a conversation</div>
      <div class="text-xs mt-2 leading-relaxed">
        The selected agent runs with full tool access in{" "}
        <span class="font-mono text-ink-100">{cwd || "~"}</span>.
        Drop, paste, or upload files to reference them.
      </div>
    </div>
  );
}
