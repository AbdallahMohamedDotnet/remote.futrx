import { useState } from "preact/hooks";
import type { ProviderKey } from "../../models/settings";

export function useSettingsCredentials() {
  const [values, setValues] = useState<Record<ProviderKey, string>>({
    github: "",
    cloudflare: "",
    hetzner: "",
    gcloud: "",
  });
  const [expandedHelp, setExpandedHelp] = useState<Record<ProviderKey, boolean>>({} as Record<ProviderKey, boolean>);
  const [revealed, setRevealed] = useState<Record<ProviderKey, boolean>>({} as Record<ProviderKey, boolean>);
  const [savedAt, setSavedAt] = useState<Record<ProviderKey, number | undefined>>({} as Record<ProviderKey, number | undefined>);

  function setValue(key: ProviderKey, value: string) {
    setValues((current) => ({ ...current, [key]: value }));
  }

  function toggleHelp(key: ProviderKey) {
    setExpandedHelp((current) => ({ ...current, [key]: !current[key] }));
  }

  function toggleReveal(key: ProviderKey) {
    setRevealed((current) => ({ ...current, [key]: !current[key] }));
  }

  function save(key: ProviderKey) {
    setSavedAt((current) => ({ ...current, [key]: Date.now() }));
    setTimeout(() => {
      setSavedAt((current) => ({ ...current, [key]: undefined }));
    }, 2000);
  }

  return {
    values,
    expandedHelp,
    revealed,
    savedAt,
    setValue,
    toggleHelp,
    toggleReveal,
    save,
  };
}
