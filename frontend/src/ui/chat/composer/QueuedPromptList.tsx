import type { QueuedPrompt } from "../../../models/chat";
import { X } from "../../primitives/icons";

export function QueuedPromptList({
  queuedPrompts,
  inflightId,
  onRemove,
}: {
  queuedPrompts: QueuedPrompt[];
  inflightId: string | null;
  onRemove: (id: string) => void;
}) {
  if (queuedPrompts.length === 0) return null;

  return (
    <div class="px-3 pb-2">
      <div class="w-full rounded-lg border border-line bg-tint p-2">
        <div class="flex items-center justify-between gap-3 px-1 pb-1.5">
          <div class="text-[12px] font-medium text-ink-200">Queue</div>
          <div class="text-[11px] text-ink-400">{queuedPrompts.length} waiting</div>
        </div>
        <div class="flex flex-wrap gap-2">
          {queuedPrompts.map((prompt, index) => (
            <div key={prompt.id} class="group min-w-0 max-w-full inline-flex items-center gap-2 rounded-md bg-surface border border-line px-2 py-1.5">
              <span class="text-[11px] text-ink-400 flex-none">
                {prompt.id === inflightId ? "sending" : `#${index + 1}`}
              </span>
              <span class="text-[12px] text-ink-100 truncate max-w-[260px] sm:max-w-[420px]" title={prompt.text}>
                {prompt.text}
              </span>
              <button
                type="button"
                onClick={() => onRemove(prompt.id)}
                disabled={prompt.id === inflightId}
                class="w-6 h-6 grid place-items-center rounded text-ink-300 hover:text-accent-red hover:bg-accent-red/10 flex-none disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:bg-transparent disabled:hover:text-ink-300"
                aria-label={prompt.id === inflightId ? "Prompt is being delivered" : "Remove queued prompt"}
                title={prompt.id === inflightId ? "Prompt is being delivered" : "Remove queued prompt"}
              >
                <X class="w-3 h-3" />
              </button>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
