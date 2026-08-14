import { useEffect, useRef, useState } from "preact/hooks";
import { ChevronDown } from "../../primitives/icons";

export function ComposerModelPicker({
  model,
  streaming,
  options,
  onChange,
}: {
  model: string;
  streaming: boolean;
  options: readonly { value: string; label: string; sub: string }[];
  onChange: (model: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const label = options.find((option) => option.value === model)?.label || model || "Auto";

  useEffect(() => {
    if (!open) return;
    function closeOnOutsideClick(event: MouseEvent) {
      const target = event.target as Node | null;
      if (target && !rootRef.current?.contains(target)) setOpen(false);
    }
    window.addEventListener("mousedown", closeOnOutsideClick);
    return () => window.removeEventListener("mousedown", closeOnOutsideClick);
  }, [open]);

  function pick(value: string) {
    setOpen(false);
    if (value !== model) onChange(value);
  }

  return (
    <div ref={rootRef} class="relative w-[150px] flex-none sm:w-[180px] md:w-[210px]">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        class={`h-8 w-full min-w-0 rounded-md px-2.5 text-left transition disabled:cursor-not-allowed disabled:opacity-60
                ${open ? "bg-accent-blue/[0.12]" : "bg-white/[0.045] hover:bg-white/[0.075]"}`}
        disabled={streaming}
        title={streaming ? "Cannot change model while streaming" : "Choose model"}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span class="flex min-w-0 items-center gap-2">
          <span class="flex-none text-[11px] font-medium text-ink-400">Model</span>
          <span class="min-w-0 flex-1 truncate text-[13px] font-semibold text-ink-100">{label}</span>
          <ChevronDown class="h-3.5 w-3.5 flex-none text-ink-300" />
        </span>
      </button>

      {open && (
        <div
          class="theme-menu-surface absolute left-0 bottom-full z-40 mb-2 w-[min(23rem,calc(100vw-1.5rem))]
                 rounded-lg border border-white/10 bg-[#14161d] p-1 shadow-2xl"
          role="listbox"
        >
          {model && !options.some((option) => option.value === model) && (
            <button
              type="button"
              onClick={() => pick(model)}
              class="w-full rounded-md bg-accent-blue/[0.14] px-3 py-2.5 text-left text-accent-blue"
              role="option"
              aria-selected="true"
            >
              <span class="block text-[13px] font-semibold">{model}</span>
              <span class="block text-[12px] text-ink-300">custom model</span>
            </button>
          )}
          {options.map((option) => {
            const active = (model || "") === option.value;
            return (
              <button
                key={option.value || "auto"}
                type="button"
                onClick={() => pick(option.value)}
                class={`w-full rounded-md px-3 py-2.5 text-left transition
                        ${active ? "bg-accent-blue/[0.14] text-accent-blue" : "text-ink-100 hover:bg-white/[0.07]"}`}
                role="option"
                aria-selected={active}
              >
                <span class="flex items-center justify-between gap-3">
                  <span class="min-w-0">
                    <span class="block truncate text-[13px] font-semibold">{option.label}</span>
                    <span class="block truncate text-[12px] text-ink-300">{option.sub}</span>
                  </span>
                  {active && <span class="h-2 w-2 flex-none rounded-full bg-accent-blue" />}
                </span>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
