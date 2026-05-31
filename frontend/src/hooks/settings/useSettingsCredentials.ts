import { useCallback, useEffect, useMemo, useState } from "preact/hooks";
import type { ProviderKey } from "../../models/settings";
import { settingsService } from "../../services/settingsService";
import { CREDENTIAL_PROVIDERS } from "../../state/settings/providers";

/**
 * Drives the credential cards on the settings page.
 *
 * Source of truth lives on the server (see backend's user-settings handler).
 * On save we POST the value under the provider's canonical env-var name and
 * the backend pushes it into every project container's `environment.<KEY>`
 * via `lxc config set` — so any subsequent shell or Claude tool call inside
 * a container sees the var automatically.
 *
 * Values themselves are never returned to the browser. The server masks
 * each set key as the string "set" on GET; we use that to drive the
 * persistent "stored on server" badge.
 */
export function useSettingsCredentials() {
  const emptyValues = useMemo<Record<ProviderKey, string>>(
    () => ({ github: "", cloudflare: "", hetzner: "", gcloud: "" }),
    [],
  );
  const emptyBoolMap = useMemo<Record<ProviderKey, boolean>>(
    () => ({} as Record<ProviderKey, boolean>),
    [],
  );

  const [values, setValues] = useState<Record<ProviderKey, string>>(emptyValues);
  const [expandedHelp, setExpandedHelp] = useState<Record<ProviderKey, boolean>>(emptyBoolMap);
  const [revealed, setRevealed] = useState<Record<ProviderKey, boolean>>(emptyBoolMap);
  const [savedAt, setSavedAt] = useState<Record<ProviderKey, number | undefined>>({} as Record<ProviderKey, number | undefined>);
  const [stored, setStored] = useState<Record<ProviderKey, boolean>>(emptyBoolMap);
  const [saving, setSaving] = useState<Record<ProviderKey, boolean>>(emptyBoolMap);
  const [errors, setErrors] = useState<Record<ProviderKey, string | undefined>>({} as Record<ProviderKey, string | undefined>);
  const [propagation, setPropagation] = useState<
    Record<ProviderKey, { propagated: number; failures: number } | undefined>
  >({} as Record<ProviderKey, { propagated: number; failures: number } | undefined>);

  // Map server `secrets` (keyed by env-var name) back to provider keys so we
  // can show a persistent "stored" badge.
  const envToProvider = useMemo(() => {
    const m = new Map<string, ProviderKey>();
    for (const p of CREDENTIAL_PROVIDERS) m.set(p.envVar, p.key);
    return m;
  }, []);

  const refresh = useCallback(async () => {
    try {
      const s = await settingsService.get();
      const next: Record<ProviderKey, boolean> = { ...emptyBoolMap };
      for (const env of Object.keys(s.secrets ?? {})) {
        const pk = envToProvider.get(env);
        if (pk) next[pk] = true;
      }
      setStored(next);
    } catch {
      // Non-fatal — the page is usable, just without the stored badges.
    }
  }, [emptyBoolMap, envToProvider]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  function setValue(key: ProviderKey, value: string) {
    setValues((current) => ({ ...current, [key]: value }));
  }

  function toggleHelp(key: ProviderKey) {
    setExpandedHelp((current) => ({ ...current, [key]: !current[key] }));
  }

  function toggleReveal(key: ProviderKey) {
    setRevealed((current) => ({ ...current, [key]: !current[key] }));
  }

  async function save(key: ProviderKey) {
    const provider = CREDENTIAL_PROVIDERS.find((p) => p.key === key);
    if (!provider) return;
    const value = values[key]?.trim();
    if (!value) return;

    setSaving((current) => ({ ...current, [key]: true }));
    setErrors((current) => ({ ...current, [key]: undefined }));

    try {
      const result = await settingsService.update({
        secrets: { set: { [provider.envVar]: value } },
      });
      setStored((current) => ({ ...current, [key]: true }));
      setValues((current) => ({ ...current, [key]: "" }));
      setSavedAt((current) => ({ ...current, [key]: Date.now() }));
      setPropagation((current) => ({
        ...current,
        [key]: {
          propagated: result.propagated ?? 0,
          failures: result.failures?.length ?? 0,
        },
      }));
      setTimeout(() => {
        setSavedAt((current) => ({ ...current, [key]: undefined }));
        setPropagation((current) => ({ ...current, [key]: undefined }));
      }, 4000);
    } catch (e) {
      setErrors((current) => ({ ...current, [key]: (e as Error).message }));
    } finally {
      setSaving((current) => ({ ...current, [key]: false }));
    }
  }

  async function clear(key: ProviderKey) {
    const provider = CREDENTIAL_PROVIDERS.find((p) => p.key === key);
    if (!provider) return;
    setSaving((current) => ({ ...current, [key]: true }));
    setErrors((current) => ({ ...current, [key]: undefined }));
    try {
      await settingsService.update({ secrets: { unset: [provider.envVar] } });
      setStored((current) => ({ ...current, [key]: false }));
      setValues((current) => ({ ...current, [key]: "" }));
    } catch (e) {
      setErrors((current) => ({ ...current, [key]: (e as Error).message }));
    } finally {
      setSaving((current) => ({ ...current, [key]: false }));
    }
  }

  return {
    values,
    expandedHelp,
    revealed,
    savedAt,
    stored,
    saving,
    errors,
    propagation,
    setValue,
    toggleHelp,
    toggleReveal,
    save,
    clear,
    refresh,
  };
}
