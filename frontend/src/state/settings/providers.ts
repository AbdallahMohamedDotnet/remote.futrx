import type { CredentialProvider } from "../../models/settings";

export const CREDENTIAL_PROVIDERS: CredentialProvider[] = [
  {
    key: "github",
    envVar: "GITHUB_TOKEN",
    name: "GitHub",
    blurb: "Personal access token for the gh CLI. Used by git push, gh pr create, gh repo clone, etc.",
    shape: "token",
    placeholder: "ghp_... or github_pat_...",
    generate: {
      url: "https://github.com/settings/tokens?type=beta",
      label: "Generate a fine-grained PAT",
    },
    steps: [
      'Open https://github.com/settings/tokens?type=beta and click "Generate new token".',
      'Token name: something memorable like "remote.futrx.dev".',
      'Repository access: select "All repositories" or pick specific ones.',
      "Permissions -> Repository -> Contents: Read and write, plus any other scopes you need.",
      "Click Generate, copy the token, and paste below.",
    ],
  },
  {
    key: "cloudflare",
    envVar: "CLOUDFLARE_API_TOKEN",
    name: "Cloudflare",
    blurb: "API token for wrangler. Lets containers deploy Workers, Pages, KV, R2, D1.",
    shape: "token",
    placeholder: "Cloudflare API token",
    generate: {
      url: "https://dash.cloudflare.com/profile/api-tokens",
      label: "Create an API token",
    },
    steps: [
      'Open https://dash.cloudflare.com/profile/api-tokens and click "Create Token".',
      'Use the "Edit Cloudflare Workers" template, or roll your own with the scopes you need.',
      "Set account and zone resources to whatever the agent should reach.",
      "Click Continue -> Create Token, copy the value, and paste below.",
    ],
  },
  {
    key: "hetzner",
    envVar: "HCLOUD_TOKEN",
    name: "Hetzner Cloud",
    blurb: "Per-project API token for hcloud. Project-scoped, so pick the Hetzner project the agent should act on.",
    shape: "token",
    placeholder: "Hetzner Cloud API token",
    generate: {
      url: "https://console.hetzner.cloud/",
      label: "Open Hetzner console",
    },
    steps: [
      "In the Hetzner Cloud console, select the project you want the agent to manage.",
      "Sidebar -> Security -> API Tokens.",
      'Generate API Token -> Read & Write -> Description: "remote.futrx.dev".',
      "Copy the token, paste below. Use one token per Hetzner project when needed.",
    ],
  },
  {
    key: "gcloud",
    envVar: "GOOGLE_APPLICATION_CREDENTIALS_JSON",
    name: "Google Cloud",
    blurb: "Service account JSON key. Avoids browser-based gcloud auth so containers do not need to re-login.",
    shape: "json",
    placeholder: '{\n  "type": "service_account",\n  "project_id": "..."\n}',
    generate: {
      url: "https://console.cloud.google.com/iam-admin/serviceaccounts",
      label: "Open IAM -> Service Accounts",
    },
    steps: [
      "In GCP Console, switch to the project you want the agent to access.",
      "IAM & Admin -> Service Accounts -> Create Service Account.",
      "Grant the roles you need.",
      "Open the service account -> Keys -> Add Key -> Create new key -> JSON -> Create.",
      "Open the downloaded JSON file and paste the entire contents below.",
    ],
  },
];
