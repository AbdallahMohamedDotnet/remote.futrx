import type { AuthState } from "../lib/useAuth";
import { MessageSquare } from "./icons";

export function LoginScreen({ auth }: { auth: AuthState }) {
  const error = new URLSearchParams(location.search).get("error");

  return (
    <div class="h-full grid place-items-center bg-ink-800 text-ink-100 p-6">
      <div class="w-full max-w-sm space-y-6 text-center">
        <div class="flex flex-col items-center gap-3">
          <MessageSquare class="w-10 h-10 text-accent-blue" />
          <div>
            <div class="text-lg font-semibold">remote.futrx.dev</div>
            <div class="text-xs text-ink-300 mt-1">Self-hosted Claude Code</div>
          </div>
        </div>

        {!auth.claimed ? (
          <p class="text-sm text-accent-yellow leading-relaxed">
            This server is <span class="font-semibold">unclaimed</span>. The first Google
            account that signs in becomes the admin. Make sure that's you before continuing.
          </p>
        )}

        <a
          href="/auth/google/login"
          class="inline-flex items-center justify-center gap-2.5 w-full
                 bg-white hover:bg-ink-50 text-ink-800 font-medium text-sm
                 rounded-md px-4 py-2.5 transition-colors"
        >
          <GoogleG class="w-4 h-4" />
          Sign in with Google
        </a>

        {error && (
          <div class="text-xs text-accent-red bg-accent-red/10 border border-accent-red/30 rounded p-2 text-left">
            {error === "not-admin"
              ? `This account isn't the admin (${auth.adminEmail || "configured admin"}).`
              : `Login error: ${error}`}
          </div>
        )}

        <div class="text-[11px] text-ink-300 leading-relaxed pt-4 border-t border-ink-700">
          Locked out? SSH to the server and remove{" "}
          <code class="font-mono text-ink-200">data/admin.json</code> — the next login claims it.
        </div>
      </div>
    </div>
  );
}

// Official-ish Google "G" mark, monochromatic.
function GoogleG(props: { class?: string }) {
  return (
    <svg viewBox="0 0 48 48" class={props.class}>
      <path fill="#FFC107" d="M43.6 20.5H42V20H24v8h11.3c-1.6 4.7-6.1 8-11.3 8-6.6 0-12-5.4-12-12s5.4-12 12-12c3.1 0 5.9 1.2 8 3l5.7-5.7C33.8 6.2 29.1 4 24 4 13 4 4 13 4 24s9 20 20 20 20-9 20-20c0-1.3-.1-2.4-.4-3.5z"/>
      <path fill="#FF3D00" d="M6.3 14.7l6.6 4.8C14.6 16 19 13 24 13c3.1 0 5.9 1.2 8 3l5.7-5.7C33.8 6.2 29.1 4 24 4 16.3 4 9.6 8.4 6.3 14.7z"/>
      <path fill="#4CAF50" d="M24 44c5 0 9.6-1.9 13.1-5l-6-5c-2 1.5-4.5 2.4-7.1 2.4-5.2 0-9.6-3.3-11.3-8l-6.5 5C9.5 39.6 16.2 44 24 44z"/>
      <path fill="#1976D2" d="M43.6 20.5H42V20H24v8h11.3c-.8 2.3-2.2 4.3-4.2 5.7l6 5c-.4.4 6.4-4.7 6.4-14.7 0-1.3-.1-2.4-.4-3.5z"/>
    </svg>
  );
}
