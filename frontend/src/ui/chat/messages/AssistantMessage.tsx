import type { AssistantMessageBlock } from "../../../models/chatMessage";
import { AssistantPartList } from "./AssistantPartList";
import { ThinkingIndicator } from "./ThinkingIndicator";

export function AssistantMessage({
  block,
  streaming,
  chatId,
  cwd,
  onAnswerQuestion,
}: {
  block: AssistantMessageBlock;
  streaming: boolean;
  chatId?: string;
  cwd?: string;
  onAnswerQuestion?: (text: string) => void;
}) {
  const reasoningActive = block.parts.at(-1)?.kind === "thinking";

  return (
    <div class="codex-assistant-block space-y-2 max-w-full">
      <AssistantPartList
        parts={block.parts}
        streaming={streaming}
        chatId={chatId}
        cwd={cwd}
        onAnswerQuestion={onAnswerQuestion}
      />
      {streaming && !block.isComplete && !reasoningActive && <ThinkingIndicator />}
    </div>
  );
}
