import type { ChatProvider } from "../../../models/chat";

export function ProviderToggle({
  provider,
  options,
  streaming,
  onChange,
}: {
  provider: ChatProvider;
  options: readonly { value: ChatProvider; label: string }[];
  streaming: boolean;
  onChange: (provider: ChatProvider) => void;
}) {
  return (
    <div class="inline-flex h-7 min-w-[132px] flex-none rounded-md bg-white/[0.045] p-0.5" aria-label="Provider">
        {options.map((option) => {
          const active = option.value === provider;
          return (
            <button
              key={option.value}
              type="button"
              onClick={() => onChange(option.value)}
              class={`h-6 flex-1 rounded px-1.5 text-[11px] font-semibold transition disabled:cursor-not-allowed disabled:opacity-60
                      ${active ? "bg-accent-blue text-white shadow-sm" : "text-ink-300 hover:bg-white/[0.07] hover:text-ink-100"}`}
              disabled={streaming}
              aria-pressed={active}
              title={streaming ? "Cannot change provider while streaming" : `Use ${option.label}`}
            >
              {option.label}
            </button>
          );
        })}
    </div>
  );
}
