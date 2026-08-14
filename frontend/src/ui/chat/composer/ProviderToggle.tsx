import type { ChatProvider } from "../../../models/chat";
import { useId } from "preact/hooks";

export function ProviderToggle({
  provider,
  options,
  streaming,
  onChange,
}: {
  provider: ChatProvider;
  options: readonly {
    value: ChatProvider;
    label: string;
    disabled?: boolean;
    disabledReason?: string;
  }[];
  streaming: boolean;
  onChange: (provider: ChatProvider) => void;
}) {
  const tooltipId = useId();

  return (
    <div class="inline-flex h-7 min-w-[132px] flex-none rounded-md bg-white/[0.045] p-0.5" aria-label="Provider">
        {options.map((option) => {
          const active = option.value === provider;
          const unavailable = !!option.disabled;
          const disabled = streaming || unavailable;
          const title = option.disabledReason
            || (streaming ? "Cannot change provider while streaming" : `Use ${option.label}`);
          return (
            <span
              key={option.value}
              class="group relative flex min-w-0 flex-1"
              tabIndex={unavailable ? 0 : undefined}
              aria-describedby={unavailable ? `${tooltipId}-${option.value}` : undefined}
            >
              <button
                type="button"
                onClick={() => onChange(option.value)}
                class={`h-6 w-full rounded px-1.5 text-[11px] font-semibold transition disabled:cursor-not-allowed disabled:opacity-60
                        ${active ? "bg-accent-blue text-white shadow-sm" : "text-ink-300 hover:bg-white/[0.07] hover:text-ink-100"}`}
                disabled={disabled}
                aria-pressed={active}
                title={title}
              >
                {option.label}
              </button>
              {unavailable && option.disabledReason && (
                <span
                  id={`${tooltipId}-${option.value}`}
                  role="tooltip"
                  class="pointer-events-none invisible absolute bottom-full left-1/2 z-50 mb-2 w-max max-w-[260px] -translate-x-1/2 rounded-md border border-white/10 bg-[#191a1f] px-2 py-1.5 text-center text-[11px] font-medium leading-snug text-ink-100 opacity-0 shadow-xl transition-opacity group-hover:visible group-hover:opacity-100 group-focus:visible group-focus:opacity-100"
                >
                  {option.disabledReason}
                </span>
              )}
            </span>
          );
        })}
    </div>
  );
}
