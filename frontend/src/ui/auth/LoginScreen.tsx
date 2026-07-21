import { MessageSquare } from "../ui/icons";

export function LoginScreen({
  claimed,
  adminEmail,
}: {
  claimed: boolean;
  adminEmail: string;
}) {
  const params = new URLSearchParams(location.search);
  const error = params.get("error");
  const errorEmail = params.get("email") ?? "";

  return (
    <div class="app-shell grid place-items-center bg-[#090b0f] text-ink-100 p-5">
      <div class="w-full max-w-sm space-y-6 text-center">
        <div class="flex flex-col items-center gap-3">
          <div class="w-14 h-14 rounded-lg bg-accent-blue/[0.14] border border-accent-blue/25 grid place-items-center">
            <MessageSquare class="w-7 h-7 text-accent-blue" />
          </div>
          <div>
            <div class="text-xl font-semibold">remote.futrx.dev</div>
            <div class="text-xs text-ink-300 mt-1">Self-hosted agent workspace</div>
          </div>
        </div>

        {!claimed && (
          <p class="text-sm text-accent-yellow leading-relaxed rounded-lg border border-accent-yellow/25 bg-accent-yellow/[0.08] p-3">
            This server is <span class="font-semibold">unclaimed</span>. The first Google
            account that signs in becomes the admin. Make sure that's you before continuing.
          </p>
        )}

        <a
          href="/auth/google/login"
          class="inline-flex items-center justify-center gap-2.5 w-full
                 bg-white hover:bg-ink-50 text-ink-800 font-medium text-sm
                 rounded-md px-4 h-11 transition-colors active:scale-[0.99]"
        >
          <GoogleG class="w-4 h-4" />
          Sign in with Google
        </a>

        {error && (
          <div class="text-xs text-accent-red bg-accent-red/10 border border-accent-red/30 rounded-lg p-3 text-left leading-relaxed">
            {error === "not-invited" ? (
              <>
                <div class="font-medium text-sm">Not invited.</div>
                <div class="mt-1">
                  {errorEmail ? <span class="font-mono">{errorEmail}</span> : "That account"} isn't
                  authorized on this server. Ask an admin to add your email, then sign in again.
                </div>
              </>
            ) : error === "not-admin" ? (
              `This account isn't the admin (${adminEmail || "configured admin"}).`
            ) : (
              `Login error: ${error}`
            )}
          </div>
        )}
      </div>
    </div>
  );
}

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
