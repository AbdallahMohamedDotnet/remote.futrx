import type { ChatProvider } from "../../../models/chat";
import { PROVIDER_OPTIONS } from "../../../state/chat/usage";

export function ProviderToggle({
  provider,
  streaming,
  onChange,
}: {
  provider: ChatProvider;
  streaming: boolean;
  onChange: (provider: ChatProvider) => void;
}) {
  return (
    <div class="flex min-w-[210px] items-center gap-2 rounded-md border border-white/10 bg-white/[0.05] px-2 py-1.5">
      <span class="flex-none text-[11px] font-medium text-ink-400">Provider</span>
      <div class="inline-flex min-w-0 flex-1 rounded-md bg-[#0b0d11] p-0.5">
        {PROVIDER_OPTIONS.map((option) => {
          const active = option.value === provider;
          return (
            <button
              key={option.value}
              type="button"
              onClick={() => onChange(option.value)}
              class={`h-7 flex-1 rounded px-2.5 text-[13px] font-semibold transition disabled:cursor-not-allowed disabled:opacity-60
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
    </div>
  );
}
