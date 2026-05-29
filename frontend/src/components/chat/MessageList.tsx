import type { RefObject } from "preact";
import type { ChatStatus } from "../../models/chat";
import type { Block } from "../../state/chat/messageBlocks";
import { Loader } from "../ui/icons";
import { MessageBlock } from "./MessageBlock";
import { ThreadEmptyState } from "./ThreadEmptyState";

export function MessageList({
  status,
  blocks,
  error,
  chatId,
  cwd,
  scrollRef,
  contentRef,
  bottomRef,
  onScroll,
  onAnswerQuestion,
  onRewind,
}: {
  status: ChatStatus;
  blocks: Block[];
  error: string | null;
  chatId: string;
  cwd?: string;
  scrollRef: RefObject<HTMLDivElement>;
  contentRef: RefObject<HTMLDivElement>;
  bottomRef: RefObject<HTMLDivElement>;
  onScroll: () => void;
  onAnswerQuestion: (text: string) => void;
  onRewind: (t: number, text: string) => void;
}) {
  return (
    <div
      ref={scrollRef}
      onScroll={onScroll}
      class="codex-message-scroll h-full overflow-y-auto touch-scroll scrollbar-thin px-3 sm:px-4 md:px-6 pt-3 md:pt-6 pb-5 md:pb-6"
    >
      <div ref={contentRef} class="mx-auto max-w-[880px] space-y-4 md:space-y-5">
        {status === "loading" && (
          <div class="flex items-center gap-2 text-ink-300 text-sm rounded-lg border border-white/10 bg-white/[0.03] px-3 py-2">
            <Loader class="w-4 h-4 animate-spin" /> Loading conversation
          </div>
        )}

        {status !== "loading" && blocks.length === 0 && <ThreadEmptyState cwd={cwd} />}

        {blocks.map((block, index) => (
          <MessageBlock
            key={`${block.type}-${block.t}-${index}`}
            block={block}
            streaming={status === "streaming" && index === blocks.length - 1}
            chatId={chatId}
            onAnswerQuestion={onAnswerQuestion}
            onRewind={onRewind}
          />
        ))}

        {error && (
          <div class="text-accent-red text-sm bg-accent-red/10 border border-accent-red/30 rounded-lg p-3">
            {error}
          </div>
        )}

        <div ref={bottomRef} class="h-px" aria-hidden="true" />
      </div>
    </div>
  );
}
