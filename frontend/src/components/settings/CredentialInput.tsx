import type { JSX } from "preact";
import type { CredentialProvider } from "../../models/settings";
import { Eye, EyeOff } from "../ui/icons";

export function CredentialInput({
  provider,
  value,
  revealed,
  onChange,
  onToggleReveal,
}: {
  provider: CredentialProvider;
  value: string;
  revealed: boolean;
  onChange: (value: string) => void;
  onToggleReveal: () => void;
}) {
  return (
    <label class="block">
      <span class="block text-[11px] uppercase tracking-wider text-ink-400 mb-1">
        {provider.shape === "json" ? "Service account JSON" : "Token"}
      </span>
      {provider.shape === "json" ? (
        <textarea
          value={value}
          onInput={(event) => onChange((event.currentTarget as HTMLTextAreaElement).value)}
          placeholder={provider.placeholder}
          spellcheck={false}
          autocomplete="off"
          rows={6}
          class={`w-full rounded-md bg-[#0b0d11] border border-white/10 text-ink-100
                  placeholder:text-ink-400 px-3 py-2.5 font-mono text-[12.5px] leading-snug
                  focus:outline-none focus:border-accent-blue resize-y min-h-[120px]
                  ${revealed ? "" : "text-security-disc"}`}
          style={revealed ? undefined : ({ WebkitTextSecurity: "disc" } as JSX.CSSProperties)}
        />
      ) : (
        <div class="relative">
          <input
            type={revealed ? "text" : "password"}
            value={value}
            onInput={(event) => onChange((event.currentTarget as HTMLInputElement).value)}
            placeholder={provider.placeholder}
            spellcheck={false}
            autocomplete="off"
            class="w-full rounded-md bg-[#0b0d11] border border-white/10 text-ink-100
                   placeholder:text-ink-400 pl-3 pr-10 h-10 font-mono text-[13px]
                   focus:outline-none focus:border-accent-blue"
          />
          <button
            type="button"
            onClick={onToggleReveal}
            class="absolute right-1 top-1 h-8 w-8 grid place-items-center text-ink-300
                   hover:text-ink-50 hover:bg-white/[0.08] rounded"
            aria-label={revealed ? "Hide token" : "Show token"}
            title={revealed ? "Hide" : "Show"}
          >
            {revealed ? <EyeOff class="w-4 h-4" /> : <Eye class="w-4 h-4" />}
          </button>
        </div>
      )}
      {provider.shape === "json" && (
        <button
          type="button"
          onClick={onToggleReveal}
          class="mt-1.5 inline-flex items-center gap-1.5 text-[12px] text-ink-300 hover:text-ink-100"
        >
          {revealed ? <EyeOff class="w-3.5 h-3.5" /> : <Eye class="w-3.5 h-3.5" />}
          {revealed ? "Hide" : "Show"} contents
        </button>
      )}
    </label>
  );
}
