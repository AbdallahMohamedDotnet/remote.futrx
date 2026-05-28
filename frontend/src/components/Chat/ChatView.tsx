import { useEffect, useMemo, useRef } from "preact/hooks";
import type { ChatMeta } from "../../types";
import { chatsApi } from "../../lib/api";
import { useChat } from "../../lib/useChat";
import { groupEvents } from "./messageBlocks";
import { MessageBlock } from "./Message";
import { ChatInput } from "./ChatInput";
import { ChatHeader } from "./ChatHeader";
import { Loader, MessageSquare } from "../icons";

interface Props {
  chat: ChatMeta;
  onHamburger: () => void;
  onMetaUpdate: () => void;
}

export function ChatView({ chat, onHamburger, onMetaUpdate }: Props) {
  const { meta, events, status, error, sendPrompt, cancel, refreshMeta } = useChat(chat.id);
  const scrollRef = useRef<HTMLDivElement>(null);
  const userScrolledRef = useRef(false);

  const blocks = useMemo(() => groupEvents(events), [events]);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    if (userScrolledRef.current) return;
    el.scrollTop = el.scrollHeight;
  }, [events.length, blocks]);

  function onScroll() {
    const el = scrollRef.current;
    if (!el) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    userScrolledRef.current = !nearBottom;
  }

  async function applyMeta(patch: { model?: string; cwd?: string; title?: string }) {
    try {
      await chatsApi.patch(chat.id, patch);
      await refreshMeta();
      onMetaUpdate();
    } catch (e) {
      alert("update failed: " + (e as Error).message);
    }
  }

  // Esc cancels streaming.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape" && status === "streaming") cancel();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [status, cancel]);

  // Prefer the freshly-loaded meta from useChat (has the live model field
  // populated after first init) but fall back to the listing copy.
  const displayMeta = meta ?? chat;

  return (
    <div class="flex-1 flex flex-col min-h-0">
      <ChatHeader
        chat={displayMeta}
        events={events}
        streaming={status === "streaming"}
        onModelChange={(m) => applyMeta({ model: m })}
        onCwdChange={(c) => applyMeta({ cwd: c })}
        onHamburger={onHamburger}
      />

      <div
        ref={scrollRef}
        onScroll={onScroll}
        class="flex-1 overflow-y-auto scrollbar-thin px-3 md:px-6 py-4 space-y-4"
      >
        {status === "loading" && (
          <div class="flex items-center gap-2 text-ink-300 text-sm">
            <Loader class="w-4 h-4 animate-spin" /> Loading conversation…
          </div>
        )}

        {status !== "loading" && blocks.length === 0 && (
          <div class="text-center text-ink-300 text-sm py-12 px-4 max-w-md mx-auto">
            <MessageSquare class="w-8 h-8 mx-auto mb-3 opacity-40" />
            <div class="font-medium text-ink-200">Start a conversation</div>
            <div class="text-xs mt-2 leading-relaxed">
              Claude runs with full tool access in
              {" "}<span class="font-mono text-ink-100">{displayMeta.cwd || "~"}</span>.
              Drop, paste, or upload files to reference them.
              Pick a model in the top-right.
            </div>
          </div>
        )}

        {blocks.map((b, i) => (
          <MessageBlock
            key={`${b.type}-${b.t}-${i}`}
            block={b}
            streaming={status === "streaming" && i === blocks.length - 1}
            chatId={chat.id}
            onAnswerQuestion={(t) => { userScrolledRef.current = false; sendPrompt(t); }}
          />
        ))}

        {error && (
          <div class="text-accent-red text-sm bg-accent-red/10 border border-accent-red/30 rounded p-2">
            {error}
          </div>
        )}
      </div>

      <ChatInput
        chatId={chat.id}
        streaming={status === "streaming"}
        onSend={(t) => { userScrolledRef.current = false; sendPrompt(t); }}
        onCancel={cancel}
        onAfterUpload={onMetaUpdate}
      />
    </div>
  );
}
