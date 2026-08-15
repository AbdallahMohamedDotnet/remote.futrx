import { useEffect, useRef, useState } from "preact/hooks";
import { ChevronDown, Loader, RotateCcw } from "../../primitives/icons";

export function ComposerModelPicker({
  model,
  streaming,
  options,
  loading,
  refreshing,
  error,
  onChange,
  onRefresh,
}: {
  model: string;
  streaming: boolean;
  options: readonly { value: string; label: string; sub: string }[];
  loading: boolean;
  refreshing: boolean;
  error: string;
  onChange: (model: string) => void;
  onRefresh: () => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const label = options.find((option) => option.value === model)?.label || model || "Auto";

  useEffect(() => {
    if (loading) setOpen(false);
  }, [loading]);

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
    <div ref={rootRef} class="codex-model-control relative w-[152px] flex-none sm:w-[168px]">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        class={`h-7 w-full min-w-0 rounded-md px-2 text-left transition disabled:cursor-not-allowed disabled:opacity-60
                ${open ? "bg-accent-blue/[0.12]" : "bg-white/[0.045] hover:bg-white/[0.075]"}`}
        disabled={streaming || loading}
        title={loading
          ? "Loading available models"
          : streaming
            ? "Cannot change model while streaming"
            : "Choose model"}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span class="flex min-w-0 items-center gap-1.5">
          <span class="sr-only">Model</span>
          {loading ? (
            <>
              <Loader class="h-3.5 w-3.5 flex-none animate-spin text-ink-300" />
              <span class="min-w-0 flex-1 truncate text-[11.5px] font-semibold text-ink-200">
                Loading models…
              </span>
            </>
          ) : (
            <>
              <span class="min-w-0 flex-1 truncate text-[11.5px] font-semibold text-ink-100">{label}</span>
              <ChevronDown class="h-3 w-3 flex-none text-ink-400" />
            </>
          )}
        </span>
      </button>

      {open && !loading && (
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
          <div class="mt-1 border-t border-white/[0.08] pt-1">
            <button
              type="button"
              onClick={() => void onRefresh()}
              disabled={refreshing}
              class="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-[12px] font-medium text-ink-300 transition hover:bg-white/[0.07] hover:text-ink-100 disabled:cursor-wait disabled:opacity-60"
            >
              {refreshing
                ? <Loader class="h-3.5 w-3.5 animate-spin" />
                : <RotateCcw class="h-3.5 w-3.5" />}
              <span>{refreshing ? "Refreshing models…" : "Refresh models"}</span>
            </button>
            {error && (
              <p class="px-3 pb-1 text-[11px] leading-4 text-red-300" role="status">
                {error}
              </p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
