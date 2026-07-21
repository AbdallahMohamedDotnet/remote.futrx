import type { RefObject } from "preact";
import { ChevronDown } from "../../ui/icons";

export function ModelPicker({
  modelRef,
  open,
  model,
  streaming,
  options,
  displayLabel,
  onToggle,
  onPick,
}: {
  modelRef: RefObject<HTMLDivElement>;
  open: boolean;
  model?: string;
  streaming: boolean;
  options: Array<{ value: string; label: string; sub: string }>;
  displayLabel: (model?: string) => string;
  onToggle: () => void;
  onPick: (model: string) => void;
}) {
  return (
    <div ref={modelRef} class="relative flex-none">
      <button
        type="button"
        onClick={onToggle}
        class="h-9 inline-flex items-center justify-center gap-1.5 text-[13px] font-medium px-2.5 sm:px-3 rounded-md
               bg-white/[0.06] hover:bg-white/10 border border-white/10 text-ink-100 disabled:opacity-50"
        disabled={streaming}
        title={streaming ? "Cannot change model while streaming" : "Switch model"}
      >
        <span>{displayLabel(model)}</span>
        <ChevronDown class="w-3.5 h-3.5 text-ink-300" />
      </button>
      {open && (
        <div class="absolute right-0 top-full mt-2 z-40 w-[220px]
                    bg-[#151922] border border-white/[0.12] rounded-lg shadow-2xl overflow-hidden p-1">
          {options.map((option) => {
            const active = (model || "") === option.value ||
              (option.value !== "" && displayLabel(model).toLowerCase() === option.value);
            return (
              <button
                key={option.value}
                type="button"
                onClick={() => onPick(option.value)}
                class={`w-full flex items-center justify-between gap-3 px-3 py-2.5 rounded-md text-left
                        ${active ? "bg-accent-blue/[0.16] text-accent-blue" : "hover:bg-white/[0.07] text-ink-100"}`}
              >
                <span>
                  <span class="block text-[14px] font-medium">{option.label}</span>
                  <span class="block text-[12px] text-ink-300">{option.sub}</span>
                </span>
                {active && <span class="h-2 w-2 rounded-full bg-accent-blue" />}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
