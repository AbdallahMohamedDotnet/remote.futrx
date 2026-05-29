import { useState } from "preact/hooks";
import type { JSX } from "preact";
import {
  AlertCircle,
  Check,
  ChevronDown,
  ChevronRight,
  ChevronLeft,
  ExternalLink,
  Eye,
  EyeOff,
  Key,
  Menu,
} from "./icons";

// Host-wide configuration page. Lets the operator paste API tokens / service
// account keys once; the backend will seed them into every project container
// so the in-container agent finds gh/wrangler/hcloud/gcloud already
// authenticated.
//
// Backend wiring is intentionally deferred — this page is read/write against
// in-memory state for now. The yellow banner makes that obvious to the user.

interface Props {
  onBack: () => void;
  onHamburger: () => void;
}

type ProviderKey = "github" | "cloudflare" | "hetzner" | "gcloud";

interface Provider {
  key: ProviderKey;
  name: string;
  blurb: string;
  // Type of input — multiline for JSON blobs, single-line for tokens.
  shape: "token" | "json";
  placeholder: string;
  // Where to generate the credential, plus a short title for the link.
  generate: { url: string; label: string };
  // Step-by-step instructions, shown when "How to generate" is expanded.
  steps: string[];
}

const PROVIDERS: Provider[] = [
  {
    key: "github",
    name: "GitHub",
    blurb: "Personal access token for the gh CLI. Used by git push, gh pr create, gh repo clone, etc.",
    shape: "token",
    placeholder: "ghp_… or github_pat_…",
    generate: {
      url: "https://github.com/settings/tokens?type=beta",
      label: "Generate a fine-grained PAT",
    },
    steps: [
      "Open https://github.com/settings/tokens?type=beta and click \"Generate new token\".",
      "Token name: something memorable like \"remote.futrx.dev\".",
      "Repository access: select \"All repositories\" or pick specific ones.",
      "Permissions → Repository → Contents: Read and write (and any other scopes you need: Pull requests, Issues, etc.).",
      "Click Generate, copy the token (starts with github_pat_…), and paste below.",
    ],
  },
  {
    key: "cloudflare",
    name: "Cloudflare",
    blurb: "API token for wrangler. Lets containers deploy Workers, Pages, KV, R2, D1.",
    shape: "token",
    placeholder: "Cloudflare API token",
    generate: {
      url: "https://dash.cloudflare.com/profile/api-tokens",
      label: "Create an API token",
    },
    steps: [
      "Open https://dash.cloudflare.com/profile/api-tokens and click \"Create Token\".",
      "Use the \"Edit Cloudflare Workers\" template (or roll your own with the scopes you need).",
      "Set account / zone resources to whatever the agent should reach.",
      "Click Continue → Create Token, copy the value (Cloudflare won't show it again), paste below.",
    ],
  },
  {
    key: "hetzner",
    name: "Hetzner Cloud",
    blurb: "Per-project API token for hcloud. Project-scoped — pick the Hetzner project you want the agent to act on.",
    shape: "token",
    placeholder: "Hetzner Cloud API token",
    generate: {
      url: "https://console.hetzner.cloud/",
      label: "Open Hetzner console",
    },
    steps: [
      "In the Hetzner Cloud console, select the project you want the agent to manage.",
      "Sidebar → Security → API Tokens.",
      "Generate API Token → Read & Write → Description: \"remote.futrx.dev\".",
      "Copy the token (shown once), paste below. To target multiple Hetzner projects, generate one token per project — only one can be stored here at a time.",
    ],
  },
  {
    key: "gcloud",
    name: "Google Cloud",
    blurb: "Service account JSON key. Avoids browser-based gcloud auth so containers don't need to re-login.",
    shape: "json",
    placeholder: "{\n  \"type\": \"service_account\",\n  \"project_id\": \"…\",\n  …\n}",
    generate: {
      url: "https://console.cloud.google.com/iam-admin/serviceaccounts",
      label: "Open IAM → Service Accounts",
    },
    steps: [
      "In GCP Console, switch to the project you want the agent to access.",
      "IAM & Admin → Service Accounts → Create Service Account.",
      "Grant the roles you need (e.g. Editor, or finer-grained like Storage Admin).",
      "Open the service account → Keys → Add Key → Create new key → JSON → Create.",
      "A JSON file downloads. Open it and paste the entire contents below.",
    ],
  },
];

export function SettingsPage({ onBack, onHamburger }: Props) {
  // Stored locally for now — backend storage will land next.
  const [values, setValues] = useState<Record<ProviderKey, string>>({
    github: "",
    cloudflare: "",
    hetzner: "",
    gcloud: "",
  });
  const [expandedHelp, setExpandedHelp] = useState<Record<ProviderKey, boolean>>({} as Record<ProviderKey, boolean>);
  const [revealed, setRevealed] = useState<Record<ProviderKey, boolean>>({} as Record<ProviderKey, boolean>);
  const [savedAt, setSavedAt] = useState<Record<ProviderKey, number | undefined>>({} as Record<ProviderKey, number | undefined>);

  function toggleHelp(k: ProviderKey) {
    setExpandedHelp((s) => ({ ...s, [k]: !s[k] }));
  }
  function toggleReveal(k: ProviderKey) {
    setRevealed((s) => ({ ...s, [k]: !s[k] }));
  }
  function save(k: ProviderKey) {
    setSavedAt((s) => ({ ...s, [k]: Date.now() }));
    // Clear the "saved" marker after a moment for honest feedback.
    setTimeout(() => {
      setSavedAt((s) => ({ ...s, [k]: undefined }));
    }, 2000);
  }

  return (
    <div class="flex-1 flex flex-col min-h-0 overflow-hidden">
      <header class="top-chrome bg-[#101318] border-b border-white/10 px-3 pb-2 flex items-center gap-2 min-h-[52px]">
        <button
          type="button"
          onClick={onHamburger}
          class="md:hidden h-10 w-10 text-ink-100 rounded-md hover:bg-white/[0.08] grid place-items-center"
          aria-label="Toggle sidebar"
        >
          <Menu class="w-5 h-5" />
        </button>
        <button
          type="button"
          onClick={onBack}
          class="hidden md:inline-flex items-center gap-1.5 h-10 px-2 text-ink-200 hover:text-ink-50
                 hover:bg-white/[0.08] rounded-md text-sm"
        >
          <ChevronLeft class="w-4 h-4" /> Chats
        </button>
        <div class="flex-1 min-w-0">
          <div class="text-[11px] text-ink-300">Host configuration</div>
          <div class="text-[15px] font-semibold text-ink-50 truncate">API credentials</div>
        </div>
      </header>

      <div class="flex-1 overflow-y-auto touch-scroll">
        <div class="max-w-2xl mx-auto px-4 py-5 space-y-4">
          <p class="text-[13px] leading-relaxed text-ink-300">
            These credentials are <strong class="text-ink-100">host-wide</strong> — paste once
            and every project's container gets seeded so <code class="font-mono text-ink-100">gh</code>,{" "}
            <code class="font-mono text-ink-100">wrangler</code>,{" "}
            <code class="font-mono text-ink-100">hcloud</code>, and{" "}
            <code class="font-mono text-ink-100">gcloud</code> Just Work inside the sandbox.
            Use long-lived tokens / service-account keys, not interactive logins.
          </p>

          <div class="flex items-start gap-2.5 rounded-lg border border-accent-yellow/30 bg-accent-yellow/[0.08]
                      text-ink-100 px-3 py-2.5 text-[13px] leading-relaxed">
            <AlertCircle class="w-4 h-4 mt-0.5 flex-none text-accent-yellow" />
            <div>
              <div class="font-medium text-accent-yellow">Backend storage not wired yet</div>
              <div class="text-ink-200 mt-0.5">
                Values entered here aren't persisted or pushed to containers — this page is the UI
                stub. The seed plumbing lands next.
              </div>
            </div>
          </div>

          {PROVIDERS.map((p) => (
            <ProviderCard
              key={p.key}
              provider={p}
              value={values[p.key]}
              onChange={(v) => setValues((s) => ({ ...s, [p.key]: v }))}
              helpOpen={!!expandedHelp[p.key]}
              onToggleHelp={() => toggleHelp(p.key)}
              revealed={!!revealed[p.key]}
              onToggleReveal={() => toggleReveal(p.key)}
              onSave={() => save(p.key)}
              savedAt={savedAt[p.key]}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

interface CardProps {
  provider: Provider;
  value: string;
  onChange: (v: string) => void;
  helpOpen: boolean;
  onToggleHelp: () => void;
  revealed: boolean;
  onToggleReveal: () => void;
  onSave: () => void;
  savedAt: number | undefined;
}

function ProviderCard({
  provider,
  value,
  onChange,
  helpOpen,
  onToggleHelp,
  revealed,
  onToggleReveal,
  onSave,
  savedAt,
}: CardProps) {
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
              <span class="inline-flex items-center gap-1 text-[11px] px-1.5 py-0.5 rounded
                           bg-accent-green/15 text-accent-green">
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
          class="inline-flex items-center gap-1.5 text-[12.5px] text-ink-200 hover:text-ink-50
                 transition-colors"
          aria-expanded={helpOpen}
        >
          {helpOpen ? <ChevronDown class="w-3.5 h-3.5" /> : <ChevronRight class="w-3.5 h-3.5" />}
          How to generate this
        </button>
        {helpOpen && (
          <div class="mt-2 ml-5 mr-1 mb-3 text-[12.5px] text-ink-200 leading-relaxed space-y-1.5">
            <ol class="list-decimal list-outside pl-4 space-y-1">
              {provider.steps.map((s, i) => (
                <li key={i}>{s}</li>
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
        <label class="block">
          <span class="block text-[11px] uppercase tracking-wider text-ink-400 mb-1">
            {provider.shape === "json" ? "Service account JSON" : "Token"}
          </span>
          {provider.shape === "json" ? (
            <textarea
              value={value}
              onInput={(e) => onChange((e.currentTarget as HTMLTextAreaElement).value)}
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
                onInput={(e) => onChange((e.currentTarget as HTMLInputElement).value)}
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
