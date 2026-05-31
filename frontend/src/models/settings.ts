export type ProviderKey = "github" | "cloudflare" | "hetzner" | "gcloud";

export interface CredentialProvider {
  key: ProviderKey;
  name: string;
  blurb: string;
  shape: "token" | "json";
  placeholder: string;
  generate: { url: string; label: string };
  steps: string[];
  /** Env-var name used inside every container. e.g. "GITHUB_TOKEN" -> `lxc config set environment.GITHUB_TOKEN`. */
  envVar: string;
}

export type AppearanceTheme = "system" | "dark" | "light";

export interface AppearanceSettings {
  theme: AppearanceTheme;
}

export interface UserSettings {
  appearance: AppearanceSettings;
  /** Server returns each key with the placeholder "set" — never the plaintext value. */
  secrets?: Record<string, string>;
  updatedAt?: number;
}

export interface SecretsUpdate {
  set?: Record<string, string>;
  unset?: string[];
}

export interface UpdateUserSettingsInput {
  appearance?: Partial<AppearanceSettings>;
  secrets?: SecretsUpdate;
}

export interface ContainerPropagationFailure {
  container: string;
  error: string;
}

export interface SettingsSaveResult extends UserSettings {
  propagated?: number;
  failures?: ContainerPropagationFailure[];
}

export const DEFAULT_USER_SETTINGS: UserSettings = {
  appearance: { theme: "system" },
};
