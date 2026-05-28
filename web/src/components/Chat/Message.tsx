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
        <div class="max-w-[80%] bg-accent-blue/15 border border-accent-blue/30
                    rounded-2xl rounded-br-md px-3.5 py-2 text-[14px] whitespace-pre-wrap break-words">
          {block.text}
        </div>
      </div>
    );
  }
  if (block.type === "error") {
    return (
      <div class="flex items-start gap-2 text-accent-red text-sm">
        <AlertCircle class="w-4 h-4 flex-none mt-0.5" />
        <div>{block.message}</div>
      </div>
    );
  }
  // assistant
  return (
    <div class="space-y-1">
      {block.parts.map((p, i) => {
        if (p.kind === "text") {
          return (
            <div key={i} class="text-[14px] leading-relaxed text-ink-100">
              <StreamingText text={p.text} streaming={streaming} />
            </div>
          );
        }
        if (p.kind === "thinking") {
          return (
            <div key={i} class="text-[12px] italic text-ink-300 border-l-2 border-ink-500 pl-3 my-2">
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
        <div class="flex items-center gap-2 text-ink-300 text-xs pt-1">
          <Loader class="w-3 h-3 animate-spin" />
          thinking…
        </div>
      )}
    </div>
  );
}
