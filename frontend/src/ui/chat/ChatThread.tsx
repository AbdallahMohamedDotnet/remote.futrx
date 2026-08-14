import type { RefObject } from "preact";
import type { ChatMeta, ChatStatus } from "../../models/chat";
import type { ChatMessageBlock } from "../../models/chatMessage";
import { ChatComposer, type ChatComposerProps } from "./composer/ChatComposer";
import { JumpToLatestButton } from "./messages/JumpToLatestButton";
import { MessageList } from "./messages/MessageList";
import { ThreadHeader } from "./header/ThreadHeader";

export function ChatThread({
  chat,
  blocks,
  hasOlder,
  loadingOlder,
  status,
  error,
  composer,
  showJump,
  scrollRef,
  contentRef,
  bottomRef,
  onHamburger,
  onScroll,
  onJumpToBottom,
  onAnswerQuestion,
  onLoadOlder,
  onRewind,
}: {
  chat: ChatMeta;
  blocks: ChatMessageBlock[];
  hasOlder: boolean;
  loadingOlder: boolean;
  status: ChatStatus;
  error: string | null;
  composer: ChatComposerProps;
  showJump: boolean;
  scrollRef: RefObject<HTMLDivElement>;
  contentRef: RefObject<HTMLDivElement>;
  bottomRef: RefObject<HTMLDivElement>;
  onHamburger: () => void;
  onScroll: () => void;
  onJumpToBottom: () => void;
  onAnswerQuestion: (text: string) => void;
  onLoadOlder: () => Promise<void>;
  onRewind: (t: number, text: string) => void;
}) {
  return (
    <div class="codex-thread flex-1 h-full flex min-h-0 overflow-hidden bg-[#0b0d11]">
      <div class="flex min-w-0 flex-1 flex-col">
        <ThreadHeader
          chat={chat}
          streaming={composer.streaming}
          onHamburger={onHamburger}
        />

        <div class="relative flex-1 min-h-0">
          <MessageList
            status={status}
            blocks={blocks}
            hasOlder={hasOlder}
            loadingOlder={loadingOlder}
            error={error}
            chatId={chat.id}
            cwd={chat.cwd}
            scrollRef={scrollRef}
            contentRef={contentRef}
            bottomRef={bottomRef}
            onScroll={onScroll}
            onAnswerQuestion={onAnswerQuestion}
            onLoadOlder={onLoadOlder}
            onRewind={onRewind}
          />
          {showJump && <JumpToLatestButton onClick={onJumpToBottom} />}
        </div>

        <ChatComposer {...composer} />
      </div>
    </div>
  );
}
