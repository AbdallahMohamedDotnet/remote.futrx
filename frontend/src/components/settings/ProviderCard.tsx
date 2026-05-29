import type { CredentialProvider } from "../../models/settings";
import { Check, ChevronDown, ChevronRight, ExternalLink, Key } from "../ui/icons";
import { CredentialInput } from "./CredentialInput";

export function ProviderCard({
  provider,
  value,
  helpOpen,
  revealed,
  savedAt,
  onChange,
  onToggleHelp,
  onToggleReveal,
  onSave,
}: {
  provider: CredentialProvider;
  value: string;
  helpOpen: boolean;
  revealed: boolean;
  savedAt: number | undefined;
  onChange: (value: string) => void;
  onToggleHelp: () => void;
  onToggleReveal: () => void;
  onSave: () => void;
}) {
  const fresh = savedAt && Date.now() - savedAt < 3000;
  const hasValue = value.trim().length > 0;

  return (
    <section class="rounded-lg border border-white/10 bg-[#101318] overflow-hidden">
      <header class="px-4 py-3 flex items-start gap-3 border-b border-white/[0.06]">
        <div class="mt-0.5 w-9 h-9 rounded-md bg-white/[0.06] border border-white/10 grid place-items-center flex-none">
          <Key class="w-4 h-4 text-ink-200" />
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <span class="text-[14.5px] font-semibold text-ink-50">{provider.name}</span>
            {hasValue && (
              <span class="inline-flex items-center gap-1 text-[11px] px-1.5 py-0.5 rounded bg-accent-green/15 text-accent-green">
                <Check class="w-3 h-3" /> set
              </span>
            )}
          </div>
          <div class="text-[12.5px] text-ink-300 mt-0.5 leading-snug">{provider.blurb}</div>
        </div>
      </header>

      <div class="px-4 pt-3 pb-1">
        <button
          type="button"
          onClick={onToggleHelp}
          class="inline-flex items-center gap-1.5 text-[12.5px] text-ink-200 hover:text-ink-50 transition-colors"
          aria-expanded={helpOpen}
        >
          {helpOpen ? <ChevronDown class="w-3.5 h-3.5" /> : <ChevronRight class="w-3.5 h-3.5" />}
          How to generate this
        </button>
        {helpOpen && (
          <div class="mt-2 ml-5 mr-1 mb-3 text-[12.5px] text-ink-200 leading-relaxed space-y-1.5">
            <ol class="list-decimal list-outside pl-4 space-y-1">
              {provider.steps.map((step, index) => (
                <li key={index}>{step}</li>
              ))}
            </ol>
            <a
              href={provider.generate.url}
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1.5 mt-2 text-accent-blue hover:underline"
            >
              <ExternalLink class="w-3.5 h-3.5" /> {provider.generate.label}
            </a>
          </div>
        )}
      </div>

      <div class="px-4 pb-4 pt-1 space-y-2">
        <CredentialInput
          provider={provider}
          value={value}
          revealed={revealed}
          onChange={onChange}
          onToggleReveal={onToggleReveal}
        />

        <div class="flex items-center gap-3 pt-1">
          <button
            type="button"
            onClick={onSave}
            disabled={!hasValue}
            class="inline-flex items-center gap-1.5 h-9 px-3 rounded-md bg-accent-blue
                   text-white text-sm font-medium hover:bg-accent-blue/90 active:scale-[0.99]
                   disabled:bg-ink-500 disabled:cursor-not-allowed transition"
          >
            Save
          </button>
          {fresh && (
            <span class="text-[12px] text-accent-green inline-flex items-center gap-1">
              <Check class="w-3.5 h-3.5" /> Stored in memory (backend wiring pending)
            </span>
          )}
        </div>
      </div>
    </section>
  );
}
