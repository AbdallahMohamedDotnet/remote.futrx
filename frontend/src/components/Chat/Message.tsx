import type { Block } from "./messageBlocks";
import { StreamingText } from "./StreamingText";
import { ToolCall } from "./ToolCall";
import { AlertCircle, Loader } from "../icons";

interface MessageBlockProps {
  block: Block;
  streaming: boolean;
  chatId?: string;
  onAnswerQuestion?: (text: string) => void;
}

export function MessageBlock({ block, streaming, chatId, onAnswerQuestion }: MessageBlockProps) {
  if (block.type === "user") {
    return (
      <div class="flex justify-end">
        <div class="max-w-[92%] sm:max-w-[78%] bg-accent-blue/15 border border-accent-blue/30
                    rounded-[18px] rounded-br-md px-3.5 py-2.5 text-[14.5px] leading-relaxed
                    whitespace-pre-wrap break-words shadow-sm">
          {block.text}
        </div>
      </div>
    );
  }
  if (block.type === "error") {
    return (
      <div class="flex items-start gap-2 text-accent-red text-sm rounded-lg border border-accent-red/25 bg-accent-red/8 px-3 py-2">
        <AlertCircle class="w-4 h-4 flex-none mt-0.5" />
        <div>{block.message}</div>
      </div>
    );
  }
  // assistant
  return (
    <div class="space-y-2 max-w-full">
      {block.parts.map((p, i) => {
        if (p.kind === "text") {
          return (
            <div key={i} class="text-[15px] leading-7 text-ink-100">
              <StreamingText text={p.text} streaming={streaming} />
            </div>
          );
        }
        if (p.kind === "thinking") {
          return (
            <div key={i} class="text-[13px] italic text-ink-300 border-l-2 border-accent-yellow/45 pl-3 my-2">
              {p.text}
            </div>
          );
        }
        return (
          <ToolCall
            key={p.id || i}
            toolUseId={p.id}
            chatId={chatId}
            name={p.name}
            input={p.input}
            output={p.output}
            isError={p.isError}
            status={p.status}
            onAnswerQuestion={onAnswerQuestion}
          />
        );
      })}
      {streaming && !block.isComplete && (
        <div class="inline-flex items-center gap-2 text-ink-300 text-xs pt-1 rounded-full bg-white/5 px-2.5 py-1">
          <Loader class="w-3 h-3 animate-spin" />
          thinking
        </div>
      )}
    </div>
  );
}
