import type { JSX } from "preact";
import { useEffect, useRef, useState } from "preact/hooks";
import { ChevronDown } from "../../primitives/icons";

export interface ComposerOption<T extends string> {
  value: T;
  label: string;
}

export function ComposerOptionDropdown<T extends string>({
  label,
  value,
  options,
  disabled = false,
  Icon,
  onChange,
}: {
  label: string;
  value: T;
  options: readonly ComposerOption<T>[];
  disabled?: boolean;
  Icon: (props: JSX.SVGAttributes<SVGSVGElement>) => JSX.Element;
  onChange: (value: T) => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const selected = options.find((option) => option.value === value) || options[0];

  useEffect(() => {
    if (!open) return;
    function closeOnOutsideClick(event: MouseEvent) {
      const target = event.target as Node | null;
      if (target && !rootRef.current?.contains(target)) setOpen(false);
    }
    window.addEventListener("mousedown", closeOnOutsideClick);
    return () => window.removeEventListener("mousedown", closeOnOutsideClick);
  }, [open]);

  function pick(nextValue: T) {
    setOpen(false);
    if (nextValue !== value) onChange(nextValue);
  }

  return (
    <div ref={rootRef} class="relative flex-none">
      <button
        type="button"
        onClick={() => setOpen((current) => !current)}
        class={`inline-flex h-7 items-center gap-1.5 rounded-md px-2 text-[11px] font-medium transition disabled:cursor-not-allowed disabled:opacity-60
                ${open ? "bg-accent-blue/[0.14] text-accent-blue" : "bg-white/[0.045] text-ink-200 hover:bg-white/[0.075] hover:text-ink-100"}`}
        disabled={disabled}
        title={`${label}: ${selected?.label || "Auto"}`}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span
          class="inline-flex h-4 w-4 flex-none items-center justify-center text-ink-400"
          title={label}
          aria-label={label}
        >
          <Icon class="h-3 w-3" />
        </span>
        <span class="sr-only">{label}</span>
        <span class="max-w-[5.5rem] truncate font-semibold text-ink-100">{selected?.label || "Auto"}</span>
        <ChevronDown class="h-3 w-3 flex-none text-ink-400" />
      </button>

      {open && (
        <div
          class="theme-menu-surface absolute left-0 bottom-full z-40 mb-1.5 w-40 rounded-lg border border-white/10 bg-[#14161d] p-1 shadow-2xl"
          role="listbox"
        >
          {options.map((option) => {
            const active = option.value === value;
            return (
              <button
                key={option.value || "auto"}
                type="button"
                onClick={() => pick(option.value)}
                class={`w-full rounded-md px-2.5 py-2 text-left text-[12px] font-medium transition
                        ${active ? "bg-accent-blue/[0.14] text-accent-blue" : "text-ink-100 hover:bg-white/[0.07]"}`}
                role="option"
                aria-selected={active}
              >
                {option.label}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
