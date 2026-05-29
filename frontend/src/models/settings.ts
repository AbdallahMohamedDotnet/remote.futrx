export type ProviderKey = "github" | "cloudflare" | "hetzner" | "gcloud";

export interface CredentialProvider {
  key: ProviderKey;
  name: string;
  blurb: string;
  shape: "token" | "json";
  placeholder: string;
  generate: { url: string; label: string };
  steps: string[];
}
