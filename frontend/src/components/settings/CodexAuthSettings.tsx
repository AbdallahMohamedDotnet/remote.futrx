import { useState } from "preact/hooks";
import { Check, Key, Loader } from "../ui/icons";

export function CodexAuthSettings({
  authenticated,
  loading,
  saving,
  error,
  onSaveAPIKey,
}: {
  authenticated: boolean;
  loading: boolean;
  saving: boolean;
  error: string | null;
  onSaveAPIKey: (apiKey: string) => Promise<void>;
}) {
  const [apiKey, setAPIKey] = useState("");
  const [localError, setLocalError] = useState<string | null>(null);

  async function submit(event: Event) {
    event.preventDefault();
    const trimmed = apiKey.trim();
    if (!trimmed) {
      setLocalError("API key is required.");
      return;
    }
    setLocalError(null);
    try {
      await onSaveAPIKey(trimmed);
      setAPIKey("");
    } catch {}
  }

  return (
    <section class="rounded-md border border-white/10 bg-white/[0.03] p-3 space-y-3">
      <div class="flex items-start gap-3">
        <div class="h-9 w-9 rounded-md bg-white/[0.06] border border-white/10 grid place-items-center flex-none text-ink-200">
          <Key class="w-4 h-4" />
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <div class="text-[14px] font-semibold text-ink-100">Codex authentication</div>
            {loading ? (
              <Loader class="w-3.5 h-3.5 text-ink-300 animate-spin" />
            ) : authenticated ? (
              <span class="inline-flex items-center gap-1 text-[11px] text-accent-green">
                <Check class="w-3.5 h-3.5" /> authenticated
              </span>
            ) : (
              <span class="text-[11px] text-ink-400">not configured</span>
            )}
          </div>
          <div class="text-[12px] text-ink-300 mt-1 leading-relaxed">
            Stores an OpenAI API-key login in <span class="font-mono text-ink-100">/root/.codex/auth.json</span>.
          </div>
        </div>
      </div>

      <form onSubmit={submit} class="grid gap-2 sm:grid-cols-[1fr_auto]">
        <input
          value={apiKey}
          onInput={(event) => setAPIKey((event.currentTarget as HTMLInputElement).value)}
          type="password"
          autocomplete="off"
          placeholder="OpenAI API key"
          class="h-10 px-3 rounded border border-white/10 bg-black/30 text-[13px] font-mono text-ink-50 placeholder-ink-400 focus:outline-none focus:border-accent-blue/50"
          disabled={saving}
        />
        <button
          type="submit"
          disabled={saving}
          class="h-10 px-3 rounded bg-accent-blue/80 hover:bg-accent-blue text-white text-[13px] font-medium disabled:opacity-50"
        >
          {saving ? "Saving..." : authenticated ? "Replace key" : "Save key"}
        </button>
      </form>

      {(localError || error) && (
        <div class="text-[12px] text-accent-red bg-accent-red/[0.08] border border-accent-red/25 rounded px-2.5 py-2 break-words">
          {localError || error}
        </div>
      )}
    </section>
  );
}
