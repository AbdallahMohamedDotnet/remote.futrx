import { useEffect, useRef, useState } from "preact/hooks";
import { claudeAuthApi } from "../lib/api";
import { Check, Loader, MessageSquare } from "./icons";

interface Props {
  onDone: () => void;
}

type Phase = "idle" | "starting" | "awaiting-code" | "submitting" | "done" | "error";

export function ClaudeLoginScreen({ onDone }: Props) {
  const [phase, setPhase] = useState<Phase>("idle");
  const [authUrl, setAuthUrl] = useState<string>("");
  const [code, setCode] = useState("");
  const [errMsg, setErrMsg] = useState<string>("");
  const codeRef = useRef<HTMLTextAreaElement>(null);

  // If the user reloads mid-flow and the backend still has an in-flight
  // session, hitting Start again replays the URL — but to be tidy we don't
  // auto-start; the button click is explicit.

  // Cancel any in-flight login when the user navigates away.
  useEffect(() => {
    return () => {
      if (phase === "starting" || phase === "awaiting-code") {
        claudeAuthApi.cancelLogin().catch(() => {});
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function startLogin() {
    setPhase("starting");
    setErrMsg("");
    try {
      const r = await claudeAuthApi.startLogin();
      setAuthUrl(r.url);
      setPhase("awaiting-code");
      setTimeout(() => codeRef.current?.focus(), 50);
    } catch (e) {
      setErrMsg((e as Error).message);
      setPhase("error");
    }
  }

  async function submitCode() {
    const trimmed = code.trim();
    if (!trimmed) return;
    setPhase("submitting");
    setErrMsg("");
    try {
      await claudeAuthApi.submitCode(trimmed);
      setPhase("done");
      setTimeout(onDone, 700);
    } catch (e) {
      setErrMsg((e as Error).message);
      setPhase("error");
    }
  }

  async function cancel() {
    try { await claudeAuthApi.cancelLogin(); } catch {}
    setPhase("idle");
    setCode("");
    setAuthUrl("");
    setErrMsg("");
  }

  return (
    <div class="h-full grid place-items-center bg-ink-800 text-ink-100 p-6">
      <div class="w-full max-w-md space-y-6">
        <div class="flex flex-col items-center gap-3 text-center">
          <div class="w-12 h-12 rounded-full bg-accent-blue/15 grid place-items-center">
            <MessageSquare class="w-6 h-6 text-accent-blue" />
          </div>
          <div>
            <div class="text-lg font-semibold">Authenticate Claude</div>
            <div class="text-xs text-ink-300 mt-1.5 leading-relaxed">
              Claude CLI is not signed into Anthropic on this server. Sign in once
              so chats can actually call the model. Tokens land in
              {" "}<code class="font-mono text-ink-100 bg-ink-700 px-1 rounded">~/.claude/.credentials.json</code>.
            </div>
          </div>
        </div>

        {phase === "idle" && (
          <button
            type="button"
            onClick={startLogin}
            class="w-full bg-accent-blue hover:bg-accent-blue/85 text-white text-sm
                   font-medium rounded-md py-2.5"
          >
            Login to Claude
          </button>
        )}

        {phase === "starting" && (
          <div class="flex items-center justify-center gap-2 text-ink-200 text-sm py-4">
            <Loader class="w-4 h-4 animate-spin" />
            Spawning <code class="font-mono text-ink-100">claude auth login</code>…
          </div>
        )}

        {phase === "awaiting-code" && (
          <div class="space-y-4">
            <ol class="text-sm text-ink-200 space-y-3 leading-relaxed">
              <li class="flex items-start gap-2">
                <span class="flex-none w-5 h-5 rounded-full bg-accent-blue/20 text-accent-blue
                             text-[11px] grid place-items-center font-semibold">1</span>
                <div>
                  Open this URL in any browser (your phone is fine) and sign in
                  with your Anthropic account:
                  <a
                    href={authUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="block mt-2 break-all text-accent-blue hover:underline font-mono text-[12px]
                           bg-ink-700 border border-ink-500 rounded p-2"
                  >
                    {authUrl}
                  </a>
                </div>
              </li>
              <li class="flex items-start gap-2">
                <span class="flex-none w-5 h-5 rounded-full bg-accent-blue/20 text-accent-blue
                             text-[11px] grid place-items-center font-semibold">2</span>
                <div>
                  On the "success" page Anthropic shows you, copy the displayed
                  code and paste it below:
                </div>
              </li>
            </ol>
            <textarea
              ref={codeRef}
              value={code}
              onInput={(e) => setCode((e.currentTarget as HTMLTextAreaElement).value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
                  e.preventDefault();
                  submitCode();
                }
              }}
              placeholder="Paste your code here"
              rows={2}
              autocomplete="off"
              autocapitalize="off"
              autocorrect="off"
              spellcheck={false}
              class="w-full resize-none rounded-md bg-ink-700 border border-ink-500
                     text-ink-100 placeholder:text-ink-300 px-3 py-2 font-mono text-[13px]
                     focus:outline-none focus:border-accent-blue"
            />
            <div class="flex gap-2">
              <button
                type="button"
                onClick={cancel}
                class="px-3 py-2 text-sm text-ink-200 hover:text-ink-100 rounded"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={submitCode}
                disabled={!code.trim()}
                class="flex-1 bg-accent-blue hover:bg-accent-blue/85
                       disabled:bg-ink-500 disabled:cursor-not-allowed
                       text-white text-sm font-medium rounded-md py-2"
              >
                Submit code
              </button>
            </div>
          </div>
        )}

        {phase === "submitting" && (
          <div class="flex items-center justify-center gap-2 text-ink-200 text-sm py-4">
            <Loader class="w-4 h-4 animate-spin" />
            Finishing up…
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
                        rounded p-2.5 whitespace-pre-wrap break-words font-mono text-[12px]">
              {errMsg}
            </div>
            <button
              type="button"
              onClick={() => { setPhase("idle"); setCode(""); setAuthUrl(""); setErrMsg(""); }}
              class="w-full bg-ink-600 hover:bg-ink-500 text-ink-100 text-sm font-medium
                     rounded-md py-2"
            >
              Try again
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
