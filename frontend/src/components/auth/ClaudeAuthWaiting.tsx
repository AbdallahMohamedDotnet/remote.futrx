import { Loader, MessageSquare } from "../ui/icons";

// Shown at the gate to non-admins when Claude isn't authenticated yet. Claude
// login is admin-only (host-wide credential), so members can't initiate it —
// they wait here until an admin signs Claude in. The status WS is live, so the
// gate opens automatically the moment that happens; no reload needed.
export function ClaudeAuthWaiting({ adminEmail }: { adminEmail?: string }) {
  return (
    <div class="app-shell grid place-items-center bg-[#090b0f] text-ink-100 p-5">
      <div class="w-full max-w-md space-y-6 text-center">
        <div class="flex flex-col items-center gap-3">
          <div class="w-14 h-14 rounded-lg bg-accent-blue/[0.14] border border-accent-blue/25 grid place-items-center">
            <MessageSquare class="w-6 h-6 text-accent-blue" />
          </div>
          <div>
            <div class="text-lg font-semibold">Waiting for Claude sign-in</div>
            <div class="text-xs text-ink-300 mt-1.5 leading-relaxed">
              Claude isn't signed into Anthropic on this server yet. An admin
              {adminEmail ? (
                <> (<span class="font-mono text-ink-100">{adminEmail}</span>)</>
              ) : null}{" "}
              needs to authenticate Claude before the workspace opens. This page
              will continue automatically once they do.
            </div>
          </div>
        </div>
        <div class="flex items-center justify-center gap-2 text-ink-300 text-sm">
          <Loader class="w-4 h-4 animate-spin" /> Listening for authentication…
        </div>
      </div>
    </div>
  );
}
