import type { RefObject } from "preact";
import type { ClaudeLoginPhase } from "../../models/auth";
import { Check, Loader, MessageSquare } from "../ui/icons";

export function ClaudeLoginScreen({
  phase,
  authUrl,
  code,
  errorMessage,
  codeRef,
  onCodeChange,
  onStartLogin,
  onSubmitCode,
  onCancel,
  onReset,
}: {
  phase: ClaudeLoginPhase;
  authUrl: string;
  code: string;
  errorMessage: string;
  codeRef: RefObject<HTMLTextAreaElement>;
  onCodeChange: (code: string) => void;
  onStartLogin: () => void;
  onSubmitCode: () => void;
  onCancel: () => void;
  onReset: () => void;
}) {
  return (
    <div class="app-shell grid place-items-center bg-[#090b0f] text-ink-100 p-5">
      <div class="w-full max-w-md space-y-6">
        <div class="flex flex-col items-center gap-3 text-center">
          <div class="w-14 h-14 rounded-lg bg-accent-blue/[0.14] border border-accent-blue/25 grid place-items-center">
            <MessageSquare class="w-6 h-6 text-accent-blue" />
          </div>
          <div>
            <div class="text-lg font-semibold">Authenticate Claude</div>
            <div class="text-xs text-ink-300 mt-1.5 leading-relaxed">
              Claude CLI is not signed into Anthropic on this server. Sign in once
              so chats can actually call the model. Tokens land in{" "}
              <code class="font-mono text-ink-100 bg-white/[0.08] px-1 rounded">~/.claude/.credentials.json</code>.
            </div>
          </div>
        </div>

        {phase === "idle" && (
          <button
            type="button"
            onClick={onStartLogin}
            class="w-full bg-accent-blue hover:bg-accent-blue/85 text-white text-sm
                   font-medium rounded-md h-11 active:scale-[0.99] transition"
          >
            Login to Claude
          </button>
        )}

        {phase === "starting" && (
          <div class="flex items-center justify-center gap-2 text-ink-200 text-sm py-4">
            <Loader class="w-4 h-4 animate-spin" />
            Spawning <code class="font-mono text-ink-100">claude auth login</code>
          </div>
        )}

        {phase === "awaiting-code" && (
          <div class="space-y-4">
            <ol class="text-sm text-ink-200 space-y-3 leading-relaxed">
              <li class="flex items-start gap-2">
                <span class="flex-none w-5 h-5 rounded-full bg-accent-blue/20 text-accent-blue
                             text-[11px] grid place-items-center font-semibold">1</span>
                <div>
                  Open this URL in any browser and sign in with your Anthropic account:
                  <a
                    href={authUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="block mt-2 break-all text-accent-blue hover:underline font-mono text-[12px]
                           bg-[#101318] border border-white/10 rounded-lg p-2.5"
                  >
                    {authUrl}
                  </a>
                </div>
              </li>
              <li class="flex items-start gap-2">
                <span class="flex-none w-5 h-5 rounded-full bg-accent-blue/20 text-accent-blue
                             text-[11px] grid place-items-center font-semibold">2</span>
                <div>Copy the displayed code from Anthropic's success page and paste it below:</div>
              </li>
            </ol>
            <textarea
              ref={codeRef}
              value={code}
              onInput={(event) => onCodeChange((event.currentTarget as HTMLTextAreaElement).value)}
              onKeyDown={(event) => {
                if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
                  event.preventDefault();
                  onSubmitCode();
                }
              }}
              placeholder="Paste your code here"
              rows={2}
              autocomplete="off"
              autocapitalize="off"
              autocorrect="off"
              spellcheck={false}
              class="w-full resize-none rounded-md bg-[#101318] border border-white/10
                     text-ink-100 placeholder:text-ink-300 px-3 py-2.5 font-mono text-[13px]
                     focus:outline-none focus:border-accent-blue"
            />
            <div class="flex gap-2">
              <button
                type="button"
                onClick={onCancel}
                class="px-3 h-10 text-sm text-ink-200 hover:text-ink-100 hover:bg-white/[0.08] rounded-md"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={onSubmitCode}
                disabled={!code.trim()}
                class="flex-1 bg-accent-blue hover:bg-accent-blue/85
                       disabled:bg-ink-500 disabled:cursor-not-allowed
                       text-white text-sm font-medium rounded-md h-10"
              >
                Submit code
              </button>
            </div>
          </div>
        )}

        {phase === "submitting" && (
          <div class="flex items-center justify-center gap-2 text-ink-200 text-sm py-4">
            <Loader class="w-4 h-4 animate-spin" />
            Finishing up
          </div>
        )}

        {phase === "done" && (
          <div class="flex items-center justify-center gap-2 text-accent-green text-sm py-4">
            <Check class="w-5 h-5" /> Claude is authenticated.
          </div>
        )}

        {phase === "error" && (
          <div class="space-y-3">
            <div class="text-accent-red text-sm bg-accent-red/10 border border-accent-red/30
                        rounded-lg p-3 whitespace-pre-wrap break-words font-mono text-[12px]">
              {errorMessage}
            </div>
            <button
              type="button"
              onClick={onReset}
              class="w-full bg-white/[0.08] hover:bg-white/[0.12] text-ink-100 text-sm font-medium
                     rounded-md h-10"
            >
              Try again
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
