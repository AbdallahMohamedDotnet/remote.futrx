export interface SegmentedOption {
  value: string;
  label: string;
}

export function SegmentedOptionGroup({
  label,
  value,
  options,
  disabled = false,
  className = "",
  onChange,
}: {
  label: string;
  value: string;
  options: SegmentedOption[];
  disabled?: boolean;
  className?: string;
  onChange: (value: string) => void;
}) {
  return (
    <div class={`flex min-w-0 max-w-full items-center gap-1.5 ${className}`}>
      <span class="flex-none text-[11px] font-medium text-ink-400">{label}</span>
      <div class="flex min-w-0 max-w-full overflow-x-auto rounded-md bg-white/[0.045] p-0.5">
        {options.map((option) => {
          const active = value === option.value;
          return (
            <button
              key={option.value || "auto"}
              type="button"
              onClick={() => onChange(option.value)}
              class={`h-7 flex-none rounded px-2.5 text-[12px] font-medium transition disabled:cursor-not-allowed disabled:opacity-60
                      ${active ? "bg-white/[0.16] text-ink-50 shadow-sm" : "text-ink-300 hover:bg-white/[0.08] hover:text-ink-100"}`}
              disabled={disabled}
              aria-pressed={active}
            >
              {option.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}
