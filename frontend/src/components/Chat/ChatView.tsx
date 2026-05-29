import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import type { ChatMeta, ChatMode } from "../../types";
import { chatsApi } from "../../lib/api";
import { useChat } from "../../lib/useChat";
import { groupEvents } from "./messageBlocks";
import { MessageBlock } from "./Message";
import { ChatInput } from "./ChatInput";
import { ChatHeader } from "./ChatHeader";
import { ArrowDown, Loader, MessageSquare } from "../icons";

interface Props {
  chat: ChatMeta;
  onHamburger: () => void;
  onMetaUpdate: () => void;
}

interface QueuedPrompt {
  id: string;
  text: string;
}

function queueId(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

export function ChatView({ chat, onHamburger, onMetaUpdate }: Props) {
  const { meta, events, status, error, canSendPrompt, sendPrompt, cancel, rewind, refreshMeta } = useChat(chat.id);
  const scrollRef = useRef<HTMLDivElement>(null);
  const userScrolledRef = useRef(false);
  const [showJump, setShowJump] = useState(false);
  const [queuedPrompts, setQueuedPrompts] = useState<QueuedPrompt[]>([]);
  const [draft, setDraft] = useState<{ key: number; text: string } | null>(null);

  const blocks = useMemo(() => groupEvents(events), [events]);

  useEffect(() => {
    setQueuedPrompts([]);
    setDraft(null);
  }, [chat.id]);

  useEffect(() => {
    if (status !== "ready" || !canSendPrompt || queuedPrompts.length === 0) return;
    const next = queuedPrompts[0];
    const sent = sendPrompt(next.text);
    if (!sent) return;
    setQueuedPrompts((prev) => prev.filter((p) => p.id !== next.id));
    userScrolledRef.current = false;
  }, [status, canSendPrompt, queuedPrompts, sendPrompt]);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    if (userScrolledRef.current) return;
    el.scrollTop = el.scrollHeight;
  }, [events.length, blocks]);

  function onScroll() {
    const el = scrollRef.current;
    if (!el) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
    userScrolledRef.current = !nearBottom;
    setShowJump(!nearBottom);
  }

  function jumpToBottom() {
    const el = scrollRef.current;
    if (!el) return;
    userScrolledRef.current = false;
    setShowJump(false);
    el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
  }

  async function applyMeta(patch: { model?: string; mode?: ChatMode; cwd?: string; title?: string }) {
    try {
      await chatsApi.patch(chat.id, patch);
      await refreshMeta();
      onMetaUpdate();
    } catch (e) {
      alert("update failed: " + (e as Error).message);
    }
  }

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape" && status === "streaming") cancel();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [status, cancel]);

  const displayMeta = meta ?? chat;
  const displayMode = displayMeta.mode || "code";

  async function handleRewind(t: number, text: string) {
    if (status === "streaming") {
      alert("Cancel the current run before rewinding this chat.");
      return;
    }
    if (!confirm("Rewind to this prompt? Messages from this point forward will be removed.")) return;
    try {
      await rewind(t);
      setQueuedPrompts([]);
      setDraft({ key: Date.now(), text });
      await refreshMeta();
      onMetaUpdate();
      userScrolledRef.current = false;
      setShowJump(false);
      setTimeout(jumpToBottom, 0);
    } catch (e) {
      alert("rewind failed: " + (e as Error).message);
    }
  }

  return (
    <div class="codex-thread flex-1 h-full flex flex-col min-h-0 overflow-hidden bg-[#0b0d11]">
      <ChatHeader
        chat={displayMeta}
        events={events}
        streaming={status === "streaming"}
        onModelChange={(m) => applyMeta({ model: m })}
        onCwdChange={(c) => applyMeta({ cwd: c })}
        onHamburger={onHamburger}
      />

      <div class="relative flex-1 min-h-0">
        <div
          ref={scrollRef}
          onScroll={onScroll}
          class="codex-message-scroll h-full overflow-y-auto touch-scroll scrollbar-thin px-3 sm:px-4 md:px-6 pt-3 md:pt-6 pb-5 md:pb-6"
        >
          <div class="mx-auto max-w-[880px] space-y-4 md:space-y-5">
            {status === "loading" && (
              <div class="flex items-center gap-2 text-ink-300 text-sm rounded-lg border border-white/10 bg-white/[0.03] px-3 py-2">
                <Loader class="w-4 h-4 animate-spin" /> Loading conversation
              </div>
            )}

            {status !== "loading" && blocks.length === 0 && (
              <div class="text-center text-ink-300 text-sm py-12 px-4 max-w-md mx-auto">
                <div class="w-14 h-14 mx-auto mb-4 rounded-lg bg-white/[0.06] border border-white/10 grid place-items-center">
                  <MessageSquare class="w-7 h-7 opacity-70" />
                </div>
                <div class="font-semibold text-ink-100 text-base">Start a conversation</div>
                <div class="text-xs mt-2 leading-relaxed">
                  Claude runs with full tool access in{" "}
                  <span class="font-mono text-ink-100">{displayMeta.cwd || "~"}</span>.
                  Drop, paste, or upload files to reference them.
                </div>
              </div>
            )}

            {blocks.map((b, i) => (
              <MessageBlock
                key={`${b.type}-${b.t}-${i}`}
                block={b}
                streaming={status === "streaming" && i === blocks.length - 1}
                chatId={chat.id}
                onAnswerQuestion={(t) => {
                  const sent = sendPrompt(t);
                  if (sent) userScrolledRef.current = false;
                }}
                onRewind={(t, text) => handleRewind(t, text)}
              />
            ))}

            {error && (
              <div class="text-accent-red text-sm bg-accent-red/10 border border-accent-red/30 rounded-lg p-3">
                {error}
              </div>
            )}
          </div>
        </div>

        {showJump && (
          <button
            type="button"
            onClick={jumpToBottom}
            class="absolute right-4 bottom-4 h-10 w-10 rounded-md bg-[#151922] border border-white/[0.12]
                   text-ink-100 shadow-xl grid place-items-center hover:bg-[#1b202b] active:scale-[0.98] transition"
            aria-label="Jump to latest message"
            title="Jump to latest"
          >
            <ArrowDown class="w-4 h-4" />
          </button>
        )}
      </div>

      <ChatInput
        chatId={chat.id}
        streaming={status === "streaming"}
        canSendPrompt={canSendPrompt}
        model={displayMeta.model || ""}
        mode={displayMode}
        queuedPrompts={queuedPrompts}
        draftText={draft?.text}
        draftKey={draft?.key}
        onModelChange={(m) => applyMeta({ model: m })}
        onModeChange={(m) => applyMeta({ mode: m })}
        onRemoveQueued={(id) => setQueuedPrompts((prev) => prev.filter((p) => p.id !== id))}
        onSend={(t) => {
          if (status === "streaming") {
            setQueuedPrompts((prev) => [...prev, { id: queueId(), text: t }]);
            return true;
          }
          const sent = sendPrompt(t);
          if (sent) userScrolledRef.current = false;
          return sent;
        }}
        onCancel={cancel}
        onAfterUpload={onMetaUpdate}
      />
    </div>
  );
}
