import { Server } from "../primitives/icons";

export function ServerInfoSettings({
  currentEmail,
  isAdmin,
  noAuth,
}: {
  currentEmail: string;
  isAdmin: boolean;
  noAuth: boolean;
}) {
  const location = typeof window === "undefined" ? null : window.location;
  const protocol = location?.protocol === "https:" ? "HTTPS (secure)" : "HTTP";
  const port = location?.port || (location?.protocol === "https:" ? "443" : "80");
  const account = noAuth ? "Local access" : currentEmail || "Signed-in user";
  const access = noAuth ? "Authentication disabled" : isAdmin ? "Administrator" : "Member";

  return (
    <section class="rounded-lg border border-white/10 bg-[#101318] overflow-hidden">
      <header class="px-4 py-3 flex items-start gap-3 border-b border-white/[0.06]">
        <div class="mt-0.5 w-9 h-9 rounded-md bg-white/[0.06] border border-white/10 grid place-items-center flex-none">
          <Server class="w-4 h-4 text-ink-200" />
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
            <div class="text-[14.5px] font-semibold text-ink-50">Parent server</div>
            <span class="inline-flex items-center gap-1.5 text-[11px] text-accent-green">
              <span class="w-1.5 h-1.5 rounded-full bg-accent-green" /> Connected
            </span>
          </div>
          <div class="text-[12.5px] text-ink-300 mt-0.5 leading-snug">
            The main host running Remote and managing all project containers.
          </div>
        </div>
      </header>

      <dl class="grid sm:grid-cols-2">
        <ServerField label="Server URL" value={location?.origin || "—"} mono />
        <ServerField label="Hostname" value={location?.hostname || "—"} mono />
        <ServerField label="Connection" value={protocol} />
        <ServerField label="Port" value={port} mono />
        <ServerField label="Account" value={account} />
        <ServerField label="Access" value={access} />
        <ServerField label="Server role" value="Parent host" />
        <ServerField label="Managed workload" value="Project containers" />
      </dl>

      <div class="border-t border-white/[0.06] px-4 py-3 text-[12.5px] leading-relaxed text-ink-300">
        Project credentials are configured per project from its Containers page, keeping each
        project's accounts and tokens isolated from the parent server settings.
      </div>
    </section>
  );
}

function ServerField({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div class="px-4 py-3 border-b border-white/[0.06] sm:odd:border-r">
      <dt class="text-[11px] uppercase tracking-wide text-ink-400">{label}</dt>
      <dd
        class={`mt-1 text-[13px] text-ink-100 break-words ${mono ? "font-mono" : ""}`}
        title={value}
      >
        {value}
      </dd>
    </div>
  );
}
